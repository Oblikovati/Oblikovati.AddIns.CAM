// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/geom2d"
)

// testFeeds is a fixed feed/height set for the generator tests.
var testFeeds = Feeds{Vert: 50, Horiz: 200, ClearanceZ: 15, SafeZ: 2}

// square returns a CCW square [0,s]×[0,s].
func square(s float64) geom2d.Polygon {
	return geom2d.Polygon{{X: 0, Y: 0}, {X: s, Y: 0}, {X: s, Y: s}, {X: 0, Y: s}}
}

// cutPolygon reconstructs the XY loop traced by the FIRST cutting pass (the first ring /
// first depth level): it collects consecutive G1 XY moves and stops at the retract that ends
// the loop, deduping the closing point, so a test can measure that loop's area.
func cutPolygon(cmds []gcode.Command) geom2d.Polygon {
	var poly geom2d.Polygon
	collecting := false
	for _, c := range cmds {
		x, hasX := c.Params["X"]
		y, hasY := c.Params["Y"]
		if c.Name == "G1" && hasX && hasY {
			poly = append(poly, geom2d.Point2{X: x, Y: y})
			collecting = true
		} else if collecting {
			break // a G0 retract ends the first loop
		}
	}
	if n := len(poly); n > 1 && poly[n-1] == poly[0] {
		poly = poly[:n-1]
	}
	return poly
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

// lastLoopArea returns the signed-free area of the last closed cut loop in the command list — the
// final contour of a multi-pass profile. Loops are the runs of G1 XY moves between retracts.
func lastLoopArea(cmds []gcode.Command) float64 {
	var loop, last geom2d.Polygon
	for _, c := range cmds {
		x, hasX := c.Params["X"]
		y, hasY := c.Params["Y"]
		if c.Name == "G1" && hasX && hasY {
			loop = append(loop, geom2d.Point2{X: x, Y: y})
			continue
		}
		if len(loop) > 0 {
			last, loop = loop, nil
		}
	}
	if len(loop) > 0 {
		last = loop
	}
	if n := len(last); n > 1 && last[n-1] == last[0] {
		last = last[:n-1]
	}
	return last.Area()
}

// TestDepthLevels covers stepped descent, the single-pass fallbacks, and exact finish.
func TestDepthLevels(t *testing.T) {
	got := DepthLevels(10, 0, 3)
	want := []float64{7, 4, 1, 0}
	if len(got) != len(want) {
		t.Fatalf("levels = %v, want %v", got, want)
	}
	for i := range want {
		if !approx(got[i], want[i]) {
			t.Errorf("level[%d] = %g, want %g", i, got[i], want[i])
		}
	}
	if l := DepthLevels(10, 0, 0); len(l) != 1 || l[0] != 0 {
		t.Errorf("zero step → %v, want [0]", l)
	}
	if l := DepthLevels(0, 5, 1); len(l) != 1 || l[0] != 5 {
		t.Errorf("inverted depths → %v, want [5]", l)
	}
}

// TestProfileOutside checks an outside contour is the boundary grown by the tool radius and
// repeated at each depth level.
func TestProfileOutside(t *testing.T) {
	levels := DepthLevels(0, -6, 3) // [-3, -6]
	cmds, err := GenerateProfile(square(10), levels, testFeeds, ProfileParams{ToolRadius: 1, Side: SideOutside, Climb: true})
	if err != nil {
		t.Fatalf("GenerateProfile: %v", err)
	}
	// Two depth levels → two plunge moves → two loop passes.
	if got := countPlunges(cmds); got != 2 {
		t.Errorf("plunge moves = %d, want 2 (one per level)", got)
	}
	// The cut loop of the first pass is the 10×10 boundary grown by radius 1 → 12×12 = 144.
	if a := cutPolygon(cmds).Area(); !approx(a, 144) {
		t.Errorf("outside profile area = %g, want 144 (12×12)", a)
	}
}

// TestProfileMultiPassOutside checks roughing passes add concentric contours that march inward to
// the final wall: three passes at 1mm step over a 10×10 part give an outer pass of 16×16 (the part
// grown by radius 1 + 2×2mm rough) down to the 12×12 finish contour, all at one Z level.
func TestProfileMultiPassOutside(t *testing.T) {
	cmds, err := GenerateProfile(square(10), []float64{0}, testFeeds, ProfileParams{
		ToolRadius: 1, Side: SideOutside, Climb: true, RoughingPasses: 3, RoughStep: 2,
	})
	if err != nil {
		t.Fatalf("GenerateProfile multi-pass: %v", err)
	}
	// Three passes at one level → three plunges (one per contour).
	if got := countPlunges(cmds); got != 3 {
		t.Errorf("multi-pass plunges = %d, want 3 (one per roughing pass)", got)
	}
	// The outermost (first) pass is the part grown by radius 1 + 2 steps of 2mm = 5 → 20×20 = 400.
	if a := cutPolygon(cmds).Area(); !approx(a, 400) {
		t.Errorf("outer roughing pass area = %g, want 400 (20×20)", a)
	}
	// The final pass must still reach the wall: the same single-pass profile is the last contour.
	single, _ := GenerateProfile(square(10), []float64{0}, testFeeds, ProfileParams{ToolRadius: 1, Side: SideOutside, Climb: true})
	last := lastLoopArea(cmds)
	if !approx(last, cutPolygon(single).Area()) {
		t.Errorf("final pass area = %g, want the single-pass wall %g", last, cutPolygon(single).Area())
	}
}

// TestProfileMultiPassSinglePassEquivalent checks one (or zero) roughing pass, or a zero step, is
// identical to the plain single-pass profile.
func TestProfileMultiPassSinglePassEquivalent(t *testing.T) {
	base := ProfileParams{ToolRadius: 1, Side: SideOutside, Climb: true}
	plain, _ := GenerateProfile(square(10), []float64{0}, testFeeds, base)
	for _, p := range []ProfileParams{
		{ToolRadius: 1, Side: SideOutside, Climb: true, RoughingPasses: 1, RoughStep: 2},
		{ToolRadius: 1, Side: SideOutside, Climb: true, RoughingPasses: 4, RoughStep: 0},
	} {
		got, _ := GenerateProfile(square(10), []float64{0}, testFeeds, p)
		if len(got) != len(plain) {
			t.Errorf("params %+v should be a single pass: got %d cmds, want %d", p, len(got), len(plain))
		}
	}
}

// TestProfileInsideTooLarge errors when the tool cannot fit the inside contour.
func TestProfileInsideTooLarge(t *testing.T) {
	if _, err := GenerateProfile(square(10), []float64{0}, testFeeds, ProfileParams{ToolRadius: 6, Side: SideInside}); err == nil {
		t.Error("an oversized tool on the inside must error")
	}
	if _, err := GenerateProfile(square(10), []float64{0}, testFeeds, ProfileParams{ToolRadius: 0, Side: SideOn}); err == nil {
		t.Error("a zero tool radius must error")
	}
}

// TestProfileOnAndDirection covers the "on" side (no compensation) and conventional milling
// (reversed winding).
func TestProfileOnAndDirection(t *testing.T) {
	on, err := GenerateProfile(square(10), []float64{0}, testFeeds, ProfileParams{ToolRadius: 1, Side: SideOn, Climb: true})
	if err != nil {
		t.Fatalf("on profile: %v", err)
	}
	if a := cutPolygon(on).Area(); !approx(a, 100) {
		t.Errorf("on-side area = %g, want 100 (no compensation)", a)
	}
	conv, _ := GenerateProfile(square(10), []float64{0}, testFeeds, ProfileParams{ToolRadius: 1, Side: SideOn, Climb: false})
	if cutPolygon(conv).IsCCW() {
		t.Error("conventional milling should reverse the loop to CW")
	}
}

// TestPocketRings checks a square pocket clears with concentric rings (one loop per ring per
// level) and that an oversized tool errors.
func TestPocketRings(t *testing.T) {
	cmds, err := GeneratePocket(square(20), []float64{0}, testFeeds, PocketParams{ToolRadius: 2, StepOver: 0.5, Climb: true})
	if err != nil {
		t.Fatalf("GeneratePocket: %v", err)
	}
	// rings at inward offsets 2,4,6,8 (offset 10 collapses) → 4 rings → 4 plunge moves.
	if got := countPlunges(cmds); got != 4 {
		t.Errorf("pocket rings = %d, want 4", got)
	}
	// The outermost ring is the boundary offset in by the radius → 16×16 = 256.
	if a := cutPolygon(cmds).Area(); !approx(a, 256) {
		t.Errorf("outer ring area = %g, want 256 (16×16)", a)
	}
	if _, err := GeneratePocket(square(2), []float64{0}, testFeeds, PocketParams{ToolRadius: 2}); err == nil {
		t.Error("a tool too large for the region must error")
	}
}

// countPlunges counts the plunge moves (G1 carrying a Z address) — one per cut loop.
func countPlunges(cmds []gcode.Command) int {
	n := 0
	for _, c := range cmds {
		if _, hasZ := c.Params["Z"]; c.Name == "G1" && hasZ {
			n++
		}
	}
	return n
}
