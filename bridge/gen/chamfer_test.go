// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// cutZ returns the depth of the first cutting (G1 with Z) move — the chamfer pass depth.
func cutZ(cmds []gcode.Command) (float64, bool) {
	for _, c := range cmds {
		if z, ok := c.Params["Z"]; ok && c.Name == "G1" {
			return z, true
		}
	}
	return 0, false
}

// TestChamferDepthFromAngle checks a 90° tool cuts a chamfer whose depth equals its width, and a
// 60° tool cuts deeper (width / tan(30°)).
func TestChamferDepthFromAngle(t *testing.T) {
	right, err := GenerateChamfer(square(20), 0, testFeeds, ChamferParams{Width: 2, ToolAngleDeg: 90, Side: SideOutside})
	if err != nil {
		t.Fatalf("GenerateChamfer 90°: %v", err)
	}
	if z, ok := cutZ(right); !ok || !approx(z, -2) {
		t.Errorf("90° chamfer depth = %g, want -2 (= width, 45° flank)", z)
	}
	sharp, err := GenerateChamfer(square(20), 0, testFeeds, ChamferParams{Width: 2, ToolAngleDeg: 60, Side: SideOutside})
	if err != nil {
		t.Fatalf("GenerateChamfer 60°: %v", err)
	}
	want := -2 / math.Tan(30*math.Pi/180)
	if z, ok := cutZ(sharp); !ok || !approx(z, want) {
		t.Errorf("60° chamfer depth = %g, want %g", z, want)
	}
}

// TestChamferOffsetSide checks the tip path is the boundary grown outward by the width on an
// outside edge (so the flank bevels the outer top edge).
func TestChamferOffsetSide(t *testing.T) {
	cmds, err := GenerateChamfer(square(10), 0, testFeeds, ChamferParams{Width: 1, Side: SideOutside, Climb: true})
	if err != nil {
		t.Fatalf("GenerateChamfer: %v", err)
	}
	// 10×10 grown by 1 on every side → 12×12 = 144.
	if a := cutPolygon(cmds).Area(); !approx(a, 144) {
		t.Errorf("outside chamfer tip-path area = %g, want 144 (12×12)", a)
	}
}

// TestChamferErrors covers the degenerate width and oversized-inside cases.
func TestChamferErrors(t *testing.T) {
	if _, err := GenerateChamfer(square(10), 0, testFeeds, ChamferParams{Width: 0, Side: SideOutside}); err == nil {
		t.Error("a zero width must error")
	}
	if _, err := GenerateChamfer(square(10), 0, testFeeds, ChamferParams{Width: 6, Side: SideInside}); err == nil {
		t.Error("an inside chamfer wider than the feature must error")
	}
}
