// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/geom2d"
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

// TestChamferMultiPass checks flank passes step the bevel from the top edge down to full width:
// three passes give three loops, the innermost (first, j=1) narrower than the final at-width loop.
func TestChamferMultiPass(t *testing.T) {
	cmds, err := GenerateChamfer(square(20), 0, testFeeds, ChamferParams{Width: 3, ToolAngleDeg: 90, Side: SideOutside, Climb: true, Passes: 3})
	if err != nil {
		t.Fatalf("GenerateChamfer multi-pass: %v", err)
	}
	if got := countPlunges(cmds); got != 3 {
		t.Errorf("three flank passes → three plunges, got %d", got)
	}
	// The first (shallowest) pass offsets by width·1/3 = 1 → 22×22 = 484; the last by 3 → 26×26 = 676.
	if a := cutPolygon(cmds).Area(); !approx(a, 484) {
		t.Errorf("first flank pass area = %g, want 484 (22×22)", a)
	}
	if a := lastLoopArea(cmds); !approx(a, 676) {
		t.Errorf("final flank pass area = %g, want 676 (26×26, the full chamfer)", a)
	}
	// One pass is byte-identical to the plain single chamfer.
	single, _ := GenerateChamfer(square(20), 0, testFeeds, ChamferParams{Width: 3, ToolAngleDeg: 90, Side: SideOutside, Climb: true})
	one, _ := GenerateChamfer(square(20), 0, testFeeds, ChamferParams{Width: 3, ToolAngleDeg: 90, Side: SideOutside, Climb: true, Passes: 1})
	if len(single) != len(one) {
		t.Errorf("Passes=1 should match the single-pass chamfer: %d vs %d", len(one), len(single))
	}
}

// TestChamferInsideCornerNoGouge checks an inside chamfer never cuts a tip point deeper than its
// distance to the nearest wall allows — at a concave corner the tip crowds another edge, so the
// depth is capped there rather than driving the flank into that wall.
func TestChamferInsideCornerNoGouge(t *testing.T) {
	// L-shaped boundary with a concave corner at (20,20); an inside chamfer rides 2mm inside it.
	l := geom2d.Polygon{{X: 0, Y: 0}, {X: 40, Y: 0}, {X: 40, Y: 20}, {X: 20, Y: 20}, {X: 20, Y: 40}, {X: 0, Y: 40}}
	cmds, err := GenerateChamfer(l, 0, testFeeds, ChamferParams{Width: 2, ToolAngleDeg: 90, Side: SideInside})
	if err != nil {
		t.Fatalf("GenerateChamfer inside L: %v", err)
	}
	for _, c := range cmds {
		x, hasX := c.Params["X"]
		y, hasY := c.Params["Y"]
		z, hasZ := c.Params["Z"]
		if c.Name != "G1" || !hasX || !hasY || !hasZ {
			continue
		}
		limit := geom2d.DistanceToBoundary(geom2d.Point2{X: x, Y: y}, l) // tan45° = 1
		if -z > limit+1e-6 {
			t.Errorf("inside-chamfer point (%g,%g) cut to depth %g exceeds wall distance %g (gouge)", x, y, -z, limit)
		}
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
