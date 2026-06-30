// SPDX-License-Identifier: GPL-2.0-only

// Command voxshot renders the CAM simulator's material-removal output to PNGs: it runs real jobs
// through the headless voxel pipeline (bridge.MaterialMesh) and draws the carved stock as a shaded
// isometric solid, so the voxel carve can be validated visually. Run: `go run ./cmd/voxshot [outdir]`.
package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"oblikovati.org/cam/bridge"
	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
	"oblikovati.org/cam/bridge/geom2d"
	"oblikovati.org/cam/bridge/plot"
)

// imageSize is the side length (pixels) of each rendered shot.
const imageSize = 900

func main() {
	outDir := "screenshots"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fail(err)
	}
	for _, sh := range shots() {
		coords, indices, res, err := bridge.MaterialMesh(sh.job)
		if err != nil {
			fail(fmt.Errorf("%s: %w", sh.name, err))
		}
		img := plot.RenderMesh(coords, indices, imageSize)
		if err := writePNG(filepath.Join(outDir, sh.name+".png"), img); err != nil {
			fail(err)
		}
		fmt.Printf("wrote %s/%s.png  (%d triangles, %.2f mm voxels)\n", outDir, sh.name, len(indices)/3, res)
	}
	writePeckProfile(outDir)
}

// writePeckProfile renders a depth-over-progress chart of a G83 deep-drilling cycle — the peck
// "woodpecker" the simulator animates, which a carved-stock image cannot show (the removed volume is
// identical to a single plunge).
func writePeckProfile(outDir string) {
	points, err := bridge.MaterialToolpath(peckHoleJob())
	if err != nil {
		fail(err)
	}
	img := plot.RenderDepthProfile(points, imageSize)
	if err := writePNG(filepath.Join(outDir, "voxel-drill-peck-profile.png"), img); err != nil {
		fail(err)
	}
	fmt.Printf("wrote %s/voxel-drill-peck-profile.png  (%d toolpath points)\n", outDir, len(points))
}

// peckHoleJob drills one deep hole with a 3 mm peck increment (G83), so the toolpath steps down and
// retracts repeatedly.
func peckHoleJob() *bridge.Job {
	drill := &bridge.DrillingOp{
		OpBase:    bridge.OpBase{OpLabel: "Peck Drill", IsActive: true, ClearanceHeight: 10, RetractHeight: 2, StartDepth: 0, FinalDepth: -18},
		PeckDepth: 3,
		Holes:     []bridge.DrillTarget{{X: 20, Y: 20, Top: 0, Bottom: -18}},
	}
	j := job(stock(40, 40, 20), drill)
	j.Tools[0].Tool = bridge.ToolBit{ShapeType: "drill", Diameter: 6}
	return j
}

// shot is one labelled carved-stock image: the job whose material removal is rendered.
type shot struct {
	name string
	job  *bridge.Job
}

// shots are the carved-stock scenes — a deliberate before/after pair. The zigzag pocket fully clears
// its floor around a standing island boss (the carve is correct); the same pocket with concentric
// offsets leaves a medial rib the simulator reveals as uncut material — a demonstration that the
// carve faithfully reflects the toolpath strategy rather than idealising it.
func shots() []shot {
	return []shot{
		{"voxel-pocket-zigzag", pocketIslandJob(gen.PocketZigzag)},
		{"voxel-pocket-offset-rib", pocketIslandJob(gen.PocketOffset)},
		{"voxel-drilling", drillingJob()},
	}
}

// drillingJob drills a 3×2 grid of through-holes — a canned-cycle (G81) operation, expanded to plunge
// motion so the simulator carves the holes.
func drillingJob() *bridge.Job {
	var holes []bridge.DrillTarget
	for _, x := range []float64{15, 35, 55} {
		for _, y := range []float64{15, 35} {
			holes = append(holes, bridge.DrillTarget{X: x, Y: y, Top: 0, Bottom: -12})
		}
	}
	drill := &bridge.DrillingOp{
		OpBase: bridge.OpBase{OpLabel: "Drill", IsActive: true, ClearanceHeight: 10, RetractHeight: 2, StartDepth: 0, FinalDepth: -12},
		Holes:  holes,
	}
	j := job(stock(70, 50, 12), drill)
	j.Tools[0].Tool = bridge.ToolBit{ShapeType: "drill", Diameter: 7}
	return j
}

// pocketIslandJob clears a rectangular pocket (in the given pattern) around a central square island,
// leaving a boss standing proud of the pocket floor.
func pocketIslandJob(pattern string) *bridge.Job {
	pocket := &bridge.PocketOp{
		OpBase:   millEnv("Pocket", -7),
		StepOver: 0.6, Climb: true, Pattern: pattern,
		Boundary: rect(10, 8, 60, 42),
		Islands:  []geom2d.Polygon{rect(30, 20, 45, 32)},
	}
	return job(stock(70, 50, 12), pocket)
}

// stock is a rectangular block with its top face at Z=0 and material extending down to −h (the CAM
// Z0-at-top convention the operations cut against).
func stock(l, w, h float64) bridge.Stock {
	return bridge.Stock{Min: gcode.Vector3{X: 0, Y: 0, Z: -h}, Max: gcode.Vector3{X: l, Y: w, Z: 0}}
}

// rect is an axis-aligned CCW rectangle (mm) from (x0,y0) to (x1,y1).
func rect(x0, y0, x1, y1 float64) geom2d.Polygon {
	return geom2d.Polygon{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}
}

// millEnv is a cut envelope from the stock top (Z=0) down to finalDepth, rapiding clear above.
func millEnv(label string, finalDepth float64) bridge.OpBase {
	return bridge.OpBase{
		OpLabel: label, IsActive: true, ToolController: 0,
		ClearanceHeight: 10, SafeHeight: 2, RetractHeight: 5,
		StartDepth: 0, FinalDepth: finalDepth,
	}
}

// job builds a single-tool (6 mm end mill) job over the stock with the given operations.
func job(stk bridge.Stock, ops ...bridge.Operation) *bridge.Job {
	j := bridge.NewJob()
	j.Stock = stk
	j.Tools = []bridge.ToolController{{
		Label: "EM6", ToolNumber: 1, VertFeed: 80, HorizFeed: 400, SpindleSpeed: 5000,
		SpindleDir: "Forward", Tool: bridge.ToolBit{ShapeType: "endmill", Diameter: 6},
	}}
	j.Operations = ops
	return j
}

// writePNG encodes an image to a file, surfacing both encode and close errors.
func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// fail prints an error and exits non-zero.
func fail(err error) {
	fmt.Fprintln(os.Stderr, "voxshot:", err)
	os.Exit(1)
}
