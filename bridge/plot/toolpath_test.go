// SPDX-License-Identifier: GPL-2.0-only

package plot

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/geom2d"
)

// TestRenderDrawsCutAndPlunge renders a plunge-then-cut path and checks the cut colour and a
// plunge marker appear, and that the image is the requested size.
func TestRenderDrawsCutAndPlunge(t *testing.T) {
	path := gcode.Path{Commands: []gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"X": 0, "Y": 0}),
		gcode.NewCommand("G1", map[string]float64{"Z": -2, "F": 100}), // plunge
		gcode.NewCommand("G1", map[string]float64{"X": 10, "Y": 0, "F": 200}),
		gcode.NewCommand("G1", map[string]float64{"X": 10, "Y": 10, "F": 200}),
	}}
	img := Render(Scene{Path: path, Boundary: geom2d.Polygon{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}}}, 200)
	if b := img.Bounds(); b.Dx() != 200 || b.Dy() != 200 {
		t.Fatalf("image size = %dx%d, want 200x200", b.Dx(), b.Dy())
	}
	cut, plunge := 0, 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			switch img.At(x, y) {
			case colCut:
				cut++
			case colPlunge:
				plunge++
			}
		}
	}
	if cut == 0 {
		t.Error("no cut-coloured pixels drawn")
	}
	if plunge == 0 {
		t.Error("no plunge marker drawn")
	}
}

// TestIsDrillCycle recognises canned cycles and rejects ordinary moves.
func TestIsDrillCycle(t *testing.T) {
	for _, name := range []string{"G81", "G83", "G73", "G85"} {
		if !isDrillCycle(gcode.NewCommand(name, nil)) {
			t.Errorf("%s should be a drill cycle", name)
		}
	}
	for _, name := range []string{"G0", "G1", "G2", "G3"} {
		if isDrillCycle(gcode.NewCommand(name, nil)) {
			t.Errorf("%s should not be a drill cycle", name)
		}
	}
}

// TestArcSweep returns a quarter turn each way.
func TestArcSweep(t *testing.T) {
	if s := arcSweep(0, math.Pi/2, true); math.Abs(s-math.Pi/2) > 1e-9 {
		t.Errorf("ccw sweep = %g, want π/2", s)
	}
	if s := arcSweep(0, math.Pi/2, false); math.Abs(s-(math.Pi/2-2*math.Pi)) > 1e-9 {
		t.Errorf("cw sweep = %g, want π/2-2π", s)
	}
}
