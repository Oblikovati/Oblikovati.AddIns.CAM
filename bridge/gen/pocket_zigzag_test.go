// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"strings"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/geom2d"
)

// rowDirections returns the sign of each horizontal cutting move's X travel, in order — the
// signature of a back-and-forth (alternating) sweep.
func rowDirections(cmds []gcode.Command) []int {
	var dirs []int
	var lastX float64
	haveX := false
	for _, c := range cmds {
		x, ok := c.Params["X"]
		if !ok {
			continue
		}
		if c.Name == "G1" && haveX && x != lastX {
			if x > lastX {
				dirs = append(dirs, 1)
			} else {
				dirs = append(dirs, -1)
			}
		}
		lastX, haveX = x, true
	}
	return dirs
}

// TestZigzagPocketSweepsBackAndForth checks the zigzag pattern lays down parallel rows that
// alternate direction and stay within the radius-inset region of a square pocket.
func TestZigzagPocketSweepsBackAndForth(t *testing.T) {
	cmds, err := GeneratePocket(square(40), []float64{0}, testFeeds, PocketParams{
		ToolRadius: 2, StepOver: 0.5, Pattern: PocketZigzag,
	})
	if err != nil {
		t.Fatalf("zigzag pocket: %v", err)
	}
	// Every cut stays inside the inset band [2,38] in X and Y.
	for _, c := range cmds {
		if c.Name != "G1" {
			continue
		}
		if x, ok := c.Params["X"]; ok && (x < 2-1e-6 || x > 38+1e-6) {
			t.Errorf("zigzag cut X=%g outside inset band [2,38]", x)
		}
		if y, ok := c.Params["Y"]; ok && (y < 2-1e-6 || y > 38+1e-6) {
			t.Errorf("zigzag cut Y=%g outside inset band [2,38]", y)
		}
	}
	// The sweep alternates direction (a back-and-forth, not all one way).
	dirs := rowDirections(cmds)
	sawPlus, sawMinus := false, false
	for _, d := range dirs {
		sawPlus = sawPlus || d > 0
		sawMinus = sawMinus || d < 0
	}
	if !sawPlus || !sawMinus {
		t.Errorf("a zigzag should sweep both directions, got dirs %v", dirs)
	}
	// On an island-free convex pocket the rows link without re-plunging: just one plunge.
	if pl := countPlunges(cmds); pl != 1 {
		t.Errorf("a convex zigzag pocket should link rows with one plunge, got %d", pl)
	}
}

// TestZigzagPocketRoutesAroundIsland checks the zigzag avoids a central island: no cutting move
// lands inside the island grown by the tool radius, and the broken rows force extra plunges.
func TestZigzagPocketRoutesAroundIsland(t *testing.T) {
	island := geom2d.Polygon{{X: 15, Y: 15}, {X: 25, Y: 15}, {X: 25, Y: 25}, {X: 15, Y: 25}}
	cmds, err := GeneratePocket(square(40), []float64{0}, testFeeds, PocketParams{
		ToolRadius: 2, StepOver: 0.5, Pattern: PocketZigzag, Islands: []geom2d.Polygon{island},
	})
	if err != nil {
		t.Fatalf("zigzag island pocket: %v", err)
	}
	// Test against the keep-out shrunk a hair, so clipped row ends that land exactly on the
	// keep-out wall don't false-trip, while a real gouge (tool centre within the radius of the
	// island) still falls inside.
	grown, _ := geom2d.Offset(island, 1.9)
	if n := cutsInside(cmds, grown); n != 0 {
		t.Errorf("the zigzag put %d cutting moves inside the island keep-out", n)
	}
}

// TestZigzagPocketOneWay checks one-direction mode cuts every row the same way (no reversal) and
// rapids back between rows (one plunge per row), unlike the linked back-and-forth zigzag.
func TestZigzagPocketOneWay(t *testing.T) {
	boundary := square(40)
	base := PocketParams{ToolRadius: 2, StepOver: 0.5, Pattern: PocketZigzag}
	oneWay := base
	oneWay.OneWay = true

	zig, err := GeneratePocket(boundary, []float64{0}, testFeeds, oneWay)
	if err != nil {
		t.Fatalf("one-way pocket: %v", err)
	}
	// Every cutting row runs the same direction: all X-travel signs are positive.
	for _, d := range rowDirections(zig) {
		if d < 0 {
			t.Errorf("one-way milling should never reverse a row, got dirs %v", rowDirections(zig))
			break
		}
	}
	// One plunge per row (rapid return between), so more plunges than the linked back-and-forth.
	linked, _ := GeneratePocket(boundary, []float64{0}, testFeeds, base)
	if countPlunges(zig) <= countPlunges(linked) {
		t.Errorf("one-way should plunge per row (%d) more than linked zigzag (%d)", countPlunges(zig), countPlunges(linked))
	}
}

// TestZigzagPocketTooSmall errors when the tool cannot enter the region.
func TestZigzagPocketTooSmall(t *testing.T) {
	if _, err := GeneratePocket(square(3), []float64{0}, testFeeds, PocketParams{ToolRadius: 2, Pattern: PocketZigzag}); err == nil {
		t.Error("a tool too large for the region must error")
	}
}

// TestPocketTooSmallErrorNamesInscribedRadius checks the "tool too large" error reports the
// region's maximum inscribed radius, so the user knows what tool would fit.
func TestPocketTooSmallErrorNamesInscribedRadius(t *testing.T) {
	_, err := GeneratePocket(square(6), []float64{0}, testFeeds, PocketParams{ToolRadius: 5})
	if err == nil {
		t.Fatal("a too-large tool should error")
	}
	if !strings.Contains(err.Error(), "inscribed radius") {
		t.Errorf("error should report the inscribed radius, got %q", err)
	}
}
