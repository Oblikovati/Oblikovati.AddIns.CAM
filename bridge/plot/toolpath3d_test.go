// SPDX-License-Identifier: GPL-2.0-only

package plot

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// TestRenderToolpath3DColours checks the 3-D backplot draws the path and uses both the cutting and
// rapid colours.
func TestRenderToolpath3DColours(t *testing.T) {
	path := gcode.NewPath([]gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"X": 0, "Y": 0, "Z": 5}),
		gcode.NewCommand("G1", map[string]float64{"Z": -3}),
		gcode.NewCommand("G1", map[string]float64{"X": 10}),
		gcode.NewCommand("G0", map[string]float64{"Z": 5}),
		gcode.NewCommand("G0", map[string]float64{"X": 0}),
	})
	img := RenderToolpath3D(path, 200)
	if img.Bounds().Dx() != 200 {
		t.Fatalf("size = %d, want 200", img.Bounds().Dx())
	}
	if !usesColor(img, depthFeed) || !usesColor(img, depthRapid) {
		t.Error("expected both cutting and rapid colours in the backplot")
	}
}

// TestRenderToolpath3DEmpty checks a path with no motion renders blank without panicking.
func TestRenderToolpath3DEmpty(t *testing.T) {
	img := RenderToolpath3D(gcode.NewPath(nil), 64)
	if img.Bounds().Dx() != 64 {
		t.Errorf("size = %d, want 64", img.Bounds().Dx())
	}
}
