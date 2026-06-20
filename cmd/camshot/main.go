// SPDX-License-Identifier: GPL-2.0-only

// Command camshot renders each CAM operation's actual generated toolpath to a PNG, producing a
// gallery that visually validates the implementation: every image is the real output of the
// operation's Execute run on a representative part, drawn as rapids (red), cuts (blue), arcs, and
// plunge points (green) over the driving boundary. Run: `go run ./cmd/camshot [outdir]`.
package main

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"oblikovati.org/cam/bridge"
	"oblikovati.org/cam/bridge/dressup"
	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
	"oblikovati.org/cam/bridge/geom2d"
	"oblikovati.org/cam/bridge/plot"
)

// imageSize is the side length (pixels) of each rendered gallery image.
const imageSize = 720

func main() {
	outDir := "screenshots"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fail(err)
	}
	for _, sh := range shots() {
		path, err := sh.op.Execute(job())
		if err != nil {
			fail(fmt.Errorf("%s: %w", sh.name, err))
		}
		img := plot.Render(plot.Scene{Path: path, Boundary: sh.boundary()}, imageSize)
		if err := writePNG(filepath.Join(outDir, sh.name+".png"), img); err != nil {
			fail(err)
		}
		fmt.Printf("wrote %s/%s.png  (%d commands)\n", outDir, sh.name, len(path.Commands))
	}
	writeProfileHoles(outDir)
}

// writeProfileHoles renders island-aware profiling: the outer outline cut outside plus an inner
// hole cut inside, combined into one toolpath — what the engine's profile job now produces.
func writeProfileHoles(outDir string) {
	outer := &bridge.ProfileOp{OpBase: millEnv("Profile"), Side: gen.SideOutside, Climb: true, Boundary: part()}
	hole := &bridge.ProfileOp{OpBase: millEnv("Hole"), Side: gen.SideInside, Climb: true, Boundary: holePoly()}
	var combined gcode.Path
	for _, op := range []*bridge.ProfileOp{outer, hole} {
		p, err := op.Execute(job())
		if err != nil {
			fail(err)
		}
		combined.Commands = append(combined.Commands, p.Commands...)
	}
	img := plot.Render(plot.Scene{Path: combined, Boundary: part()}, imageSize)
	if err := writePNG(filepath.Join(outDir, "profile-holes.png"), img); err != nil {
		fail(err)
	}
	fmt.Printf("wrote %s/profile-holes.png  (%d commands)\n", outDir, len(combined.Commands))
}

// holePoly is a small square hole (mm) inside the sample part, for the island-aware profile shot.
func holePoly() geom2d.Polygon {
	return geom2d.Polygon{{X: 22, Y: 22}, {X: 32, Y: 22}, {X: 32, Y: 32}, {X: 22, Y: 32}}
}

// squarePoly returns a CCW square [0,s]×[0,s] (mm).
func squarePoly(s float64) geom2d.Polygon {
	return geom2d.Polygon{{X: 0, Y: 0}, {X: s, Y: 0}, {X: s, Y: s}, {X: 0, Y: s}}
}

// islandPoly is a central square island (mm) the pocket clearing must route around.
func islandPoly() geom2d.Polygon {
	return geom2d.Polygon{{X: 15, Y: 15}, {X: 25, Y: 15}, {X: 25, Y: 25}, {X: 15, Y: 25}}
}

// shot is one labelled gallery entry: an operation and the boundary it renders over.
type shot struct {
	name string
	op   bridge.Operation
}

// boundary returns the driving boundary of the shot's operation for the backdrop, or nil.
func (s shot) boundary() geom2d.Polygon { return boundaryOf(s.op) }

// shots builds one entry per CAM feature on the sample part: the 2.5D clearing/contour ops, the
// four entry/relief dressups on a profile, and drilling.
func shots() []shot {
	return []shot{
		{"profile", profileOp(nil)},
		{"pocket", &bridge.PocketOp{OpBase: millEnv("Pocket"), StepOver: 0.5, Climb: true, Boundary: part()}},
		{"pocket-island", &bridge.PocketOp{OpBase: millEnv("Pocket"), StepOver: 0.5, Climb: true, Boundary: squarePoly(40), Islands: []geom2d.Polygon{islandPoly()}}},
		{"adaptive", &bridge.AdaptiveOp{OpBase: millEnv("Adaptive"), Climb: true, Boundary: part()}},
		{"adaptive-island", &bridge.AdaptiveOp{OpBase: millEnv("Adaptive"), StepOver: 0.2, Climb: true, Boundary: squarePoly(40), Islands: []geom2d.Polygon{islandPoly()}}},
		{"rest", &bridge.RestOp{OpBase: millEnv("Rest"), PrevToolDiameter: 16, StepOver: 0.5, Climb: true, Boundary: part()}},
		{"rest-island", &bridge.RestOp{OpBase: millEnv("Rest"), PrevToolDiameter: 8, StepOver: 0.5, Climb: true, Boundary: squarePoly(40), Islands: []geom2d.Polygon{islandPoly()}}},
		{"trochoidal", &bridge.TrochoidalOp{OpBase: millEnv("Trochoidal"), LoopRadius: 3, Advance: 2.5, Side: gen.SideOutside, Boundary: part()}},
		{"slot", &bridge.SlotOp{OpBase: millEnv("Slot"), Width: 10, StepOver: 0.6, Climb: true, Boundary: part()}},
		{"millface", &bridge.MillFaceOp{OpBase: millEnv("Face"), StepOver: 0.6, Boundary: part()}},
		{"engrave", &bridge.EngraveOp{OpBase: millEnv("Engrave"), Climb: true, Boundary: part()}},
		{"chamfer", &bridge.ChamferOp{OpBase: millEnv("Chamfer"), Width: 1.5, ToolAngle: 90, Side: gen.SideOutside, Climb: true, Boundary: part()}},
		{"vcarve", &bridge.VCarveOp{OpBase: millEnv("V-Carve"), ToolAngle: 90, StepOver: 0.4, Boundary: part()}},
		{"dressup-tabs", profileOp([]bridge.Dressup{bridge.NewTagsDressup(4, 3, 1)})},
		{"dressup-dogbone", profileOp([]bridge.Dressup{bridge.NewDogboneDressup(dressup.StyleDogbone, 2, 0.785, dressup.SideBoth)})},
		{"dressup-ramp", profileOp([]bridge.Dressup{bridge.NewRampDressup(4, 0.26)})},
		{"dressup-leadinout", profileOp([]bridge.Dressup{bridge.NewLeadInOutDressup(2, dressup.SideLeft)})},
		{"drilling", &bridge.DrillingOp{OpBase: millEnv("Drilling"), Holes: holes()}},
		{"probe", &bridge.ProbeOp{OpBase: deepEnv("Probe"), ProbeFeed: 50, Points: cornerProbe()}},
		{"boreprobe", &bridge.ProbeOp{OpBase: deepEnv("Bore Probe"), ProbeFeed: 50, Points: boreProbe()}},
		{"bossprobe", &bridge.ProbeOp{OpBase: deepEnv("Boss Probe"), ProbeFeed: 50, Points: bossProbe()}},
		// tool-length probing is a single-point Z measurement at the tool-setter — it has no
		// top-down toolpath shape, so it is validated by tests rather than a gallery image.
		{"helix", &bridge.HelixOp{OpBase: deepEnv("Helix"), HoleRadius: 8, Pitch: 1.5, Direction: gen.HelixCW, Holes: boreHole()}},
		{"threadmill", &bridge.ThreadMillOp{OpBase: deepEnv("Thread"), MajorDiameter: 16, Pitch: 1.5, Internal: true, Climb: true, Holes: boreHole()}},
		{"counterbore", &bridge.CounterboreOp{OpBase: deepEnv("Counterbore"), Diameter: 14, Depth: 4, Pitch: 1, Holes: boreHole()}},
		{"tapping", &bridge.TappingOp{OpBase: millEnv("Tapping"), Pitch: 1.5, Holes: holes()}},
		{"countersink", &bridge.CountersinkOp{OpBase: deepEnv("Countersink"), Diameter: 14, ToolAngle: 90, Holes: boreHole()}},
		{"surface", &bridge.SurfaceOp{OpBase: deepEnv("Surface"), Zigzag: true, Rows: pyramidRows()}},
		{"waterline", &bridge.WaterlineOp{OpBase: deepEnv("Waterline"), Levels: pyramidLevels()}},
	}
}

// The 3D-finishing shots run on a synthetic pyramid surface — apex at the centre (z=0) sloping
// down to z=-4 at the ±20mm edges — so the surface/waterline toolpaths render without a mesh or
// the OpenCAMLib drop-cutter (which needs cgo).
const (
	pyramidHalf = 20.0 // mm — half-width of the pyramid base
	pyramidDrop = 4.0  // mm — depth at the base edge below the apex
)

// pyramidHeight is the surface Z at (x,y): 0 at the centre, −pyramidDrop at the base edge.
func pyramidHeight(x, y float64) float64 {
	d := math.Max(math.Abs(x), math.Abs(y))
	if d > pyramidHalf {
		d = pyramidHalf
	}
	return -pyramidDrop * d / pyramidHalf
}

// pyramidRows samples the pyramid along parallel X scan lines — the drop-cutter rows a surface
// finish consumes.
func pyramidRows() [][]gcode.Vector3 {
	var rows [][]gcode.Vector3
	for y := -pyramidHalf; y <= pyramidHalf; y += 3 {
		var row []gcode.Vector3
		for x := -pyramidHalf; x <= pyramidHalf; x += 2 {
			row = append(row, gcode.Vector3{X: x, Y: y, Z: pyramidHeight(x, y)})
		}
		rows = append(rows, row)
	}
	return rows
}

// pyramidLevels gives the constant-Z square contours of the pyramid at three heights — the
// level loops a waterline finish consumes (nested squares growing toward the base).
func pyramidLevels() []gen.LevelLoops {
	var levels []gen.LevelLoops
	for _, z := range []float64{-1, -2, -3} {
		d := -z / pyramidDrop * pyramidHalf // half-size of the square section at height z
		loop := []gcode.Vector3{{X: -d, Y: -d, Z: z}, {X: d, Y: -d, Z: z}, {X: d, Y: d, Z: z}, {X: -d, Y: d, Z: z}}
		levels = append(levels, gen.LevelLoops{Z: z, Loops: [][]gcode.Vector3{loop}})
	}
	return levels
}

// cornerProbe is a three-touch corner cycle (Z top-off plus two edge probes) on the sample
// part's bounding box, for the probe shot.
func cornerProbe() []gen.ProbePoint {
	return []gen.ProbePoint{
		{Approach: gcode.Vector3{X: 8, Y: 8, Z: 5}, Target: gcode.Vector3{X: 8, Y: 8, Z: -3}},   // probe down
		{Approach: gcode.Vector3{X: -5, Y: 8, Z: -2}, Target: gcode.Vector3{X: 6, Y: 8, Z: -2}}, // probe +X
		{Approach: gcode.Vector3{X: 8, Y: -5, Z: -2}, Target: gcode.Vector3{X: 8, Y: 6, Z: -2}}, // probe +Y
	}
}

// boreProbe is a four-touch bore-centre cycle: from a hole centre, probe outward to the wall in
// +X, −X, +Y, −Y, for the bore-probe shot.
func boreProbe() []gen.ProbePoint {
	centre := gcode.Vector3{X: 12, Y: 12, Z: -3}
	var pts []gen.ProbePoint
	for _, d := range [][2]float64{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		pts = append(pts, gen.ProbePoint{
			Approach: centre,
			Target:   gcode.Vector3{X: 12 + d[0]*10, Y: 12 + d[1]*10, Z: -3},
		})
	}
	return pts
}

// bossProbe is a four-touch boss-centre cycle: probe inward from outside a 24mm-square boss in
// +X/−X/+Y/−Y toward its centre, for the boss-probe shot.
func bossProbe() []gen.ProbePoint {
	const half = 12
	centre := gcode.Vector3{X: 0, Y: 0, Z: -3}
	var pts []gen.ProbePoint
	for _, d := range [][2]float64{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		pts = append(pts, gen.ProbePoint{
			Approach: gcode.Vector3{X: d[0] * (half + 5), Y: d[1] * (half + 5), Z: -3},
			Target:   centre,
		})
	}
	return pts
}

// boreHole is a single central bored/tapped hole (mm) for the helix and thread-mill shots.
func boreHole() []bridge.DrillTarget {
	return []bridge.DrillTarget{{X: 0, Y: 0, Top: 0, Bottom: -8}}
}

// deepEnv is the envelope for the hole-boring ops (clearance above the stock top).
func deepEnv(label string) bridge.OpBase {
	e := millEnv(label)
	e.ClearanceHeight = 15
	return e
}

// profileOp builds an outside-contour profile over the sample part carrying the given dressups.
func profileOp(ds []bridge.Dressup) *bridge.ProfileOp {
	base := millEnv("Profile")
	base.Dressups = ds
	return &bridge.ProfileOp{OpBase: base, Side: "outside", Climb: true, StepDown: 3, Boundary: part()}
}

// boundaryOf returns the driving boundary an operation renders over (nil for drilling, which has
// no contour).
func boundaryOf(op bridge.Operation) geom2d.Polygon {
	switch o := op.(type) {
	case *bridge.ProfileOp:
		return o.Boundary
	case *bridge.PocketOp:
		return o.Boundary
	case *bridge.AdaptiveOp:
		return o.Boundary
	case *bridge.RestOp:
		return o.Boundary
	case *bridge.TrochoidalOp:
		return o.Boundary
	case *bridge.SlotOp:
		return o.Boundary
	case *bridge.MillFaceOp:
		return o.Boundary
	case *bridge.EngraveOp:
		return o.Boundary
	case *bridge.ChamferOp:
		return o.Boundary
	case *bridge.VCarveOp:
		return o.Boundary
	default:
		return nil
	}
}

// part is the sample part outline: an L-shape (mm) whose concave corner exercises the clearing
// and rest strategies.
func part() geom2d.Polygon {
	return geom2d.Polygon{
		{X: 0, Y: 0}, {X: 40, Y: 0}, {X: 40, Y: 16},
		{X: 16, Y: 16}, {X: 16, Y: 40}, {X: 0, Y: 40},
	}
}

// holes are the sample drill targets (mm), through a 10mm-thick part.
func holes() []bridge.DrillTarget {
	return []bridge.DrillTarget{
		{X: 8, Y: 8, Top: 0, Bottom: -10},
		{X: 32, Y: 8, Top: 0, Bottom: -10},
		{X: 8, Y: 32, Top: 0, Bottom: -10},
	}
}

// millEnv is the depth/height envelope shared by the gallery ops (cut 3mm deep from the top).
func millEnv(label string) bridge.OpBase {
	return bridge.OpBase{
		OpLabel: label, IsActive: true, ToolController: 0,
		ClearanceHeight: 15, SafeHeight: 2, RetractHeight: 12,
		StartDepth: 0, FinalDepth: -3,
	}
}

// job is the single-tool job (a 4mm end mill) the gallery operations resolve against.
func job() *bridge.Job {
	j := bridge.NewJob()
	j.Tools = []bridge.ToolController{{
		Label: "EM4", ToolNumber: 1, VertFeed: 60, HorizFeed: 300, SpindleSpeed: 5000,
		SpindleDir: "Forward", Tool: bridge.ToolBit{ShapeType: "endmill", Diameter: 4},
	}}
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
	fmt.Fprintln(os.Stderr, "camshot:", err)
	os.Exit(1)
}
