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
	"os"
	"path/filepath"

	"oblikovati.org/cam/bridge"
	"oblikovati.org/cam/bridge/dressup"
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
		{"adaptive", &bridge.AdaptiveOp{OpBase: millEnv("Adaptive"), Climb: true, Boundary: part()}},
		{"rest", &bridge.RestOp{OpBase: millEnv("Rest"), PrevToolDiameter: 16, StepOver: 0.5, Climb: true, Boundary: part()}},
		{"millface", &bridge.MillFaceOp{OpBase: millEnv("Face"), StepOver: 0.6, Boundary: part()}},
		{"engrave", &bridge.EngraveOp{OpBase: millEnv("Engrave"), Climb: true, Boundary: part()}},
		{"dressup-tabs", profileOp([]bridge.Dressup{bridge.NewTagsDressup(4, 3, 1)})},
		{"dressup-dogbone", profileOp([]bridge.Dressup{bridge.NewDogboneDressup(dressup.StyleDogbone, 2, 0.785, dressup.SideBoth)})},
		{"dressup-ramp", profileOp([]bridge.Dressup{bridge.NewRampDressup(4, 0.26)})},
		{"dressup-leadinout", profileOp([]bridge.Dressup{bridge.NewLeadInOutDressup(2, dressup.SideLeft)})},
		{"drilling", &bridge.DrillingOp{OpBase: millEnv("Drilling"), Holes: holes()}},
		{"helix", &bridge.HelixOp{OpBase: deepEnv("Helix"), HoleRadius: 8, Pitch: 1.5, Direction: gen.HelixCW, Holes: boreHole()}},
		{"threadmill", &bridge.ThreadMillOp{OpBase: deepEnv("Thread"), MajorDiameter: 16, Pitch: 1.5, Internal: true, Climb: true, Holes: boreHole()}},
	}
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
	case *bridge.MillFaceOp:
		return o.Boundary
	case *bridge.EngraveOp:
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
