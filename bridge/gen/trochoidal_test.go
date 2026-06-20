// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/geom2d"
)

// TestTrochoidalLoops checks the pass is a chain of full circles (two arcs each) marching along
// the contour: more, smaller loops than a plain profile, one plunge per level.
func TestTrochoidalLoops(t *testing.T) {
	cmds, err := GenerateTrochoidal(square(40), []float64{-2}, testFeeds, TrochoidalParams{
		ToolRadius: 2, LoopRadius: 3, Advance: 4, Side: SideOutside, Climb: true,
	})
	if err != nil {
		t.Fatalf("GenerateTrochoidal: %v", err)
	}
	arcs := 0
	for _, c := range cmds {
		if c.Name == "G3" {
			arcs++
		}
		if c.Name == "G2" {
			t.Errorf("climb trochoid should be all G3, found a G2")
		}
	}
	// the compensated square (44 perimeter ×4 = 176mm) at 4mm advance → ~44 loops × 2 arcs.
	if arcs < 40 {
		t.Errorf("got %d arcs, want many overlapping loops (≥40)", arcs)
	}
	if p := countPlunges(cmds); p != 1 {
		t.Errorf("trochoidal plunges = %d, want 1 (stays down through the loops)", p)
	}
}

// TestTrochoidalLoopGeometry checks each loop is a circle of LoopRadius about its centre — the
// arc endpoints sit one radius east/west of the centre.
func TestTrochoidalLoopGeometry(t *testing.T) {
	cmds, err := GenerateTrochoidal(square(20), []float64{0}, testFeeds, TrochoidalParams{
		ToolRadius: 1, LoopRadius: 2, Advance: 5, Side: SideOn, Climb: true,
	})
	if err != nil {
		t.Fatalf("GenerateTrochoidal: %v", err)
	}
	// the first loop centre is the boundary start (0,0) on the "on" side: east arc ends at x=+2.
	var firstArc geom2d.Point2
	found := false
	for _, c := range cmds {
		if c.Name == "G3" {
			firstArc = geom2d.Point2{X: c.Params["X"], Y: c.Params["Y"]}
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no loop arcs emitted")
	}
	// the west arc endpoint of the first loop is one loop-radius from the centre.
	if math.Abs(math.Hypot(firstArc.X-0, firstArc.Y-0)-2) > 1e-9 {
		t.Errorf("first arc endpoint %v is not one loop radius (2) from the centre (0,0)", firstArc)
	}
}

// TestTrochoidalErrors covers the degenerate loop/advance cases.
func TestTrochoidalErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		p    TrochoidalParams
	}{
		{"zero loop radius", TrochoidalParams{LoopRadius: 0, Advance: 4, Side: SideOn}},
		{"zero advance", TrochoidalParams{LoopRadius: 3, Advance: 0, Side: SideOn}},
	} {
		if _, err := GenerateTrochoidal(square(20), []float64{0}, testFeeds, tc.p); err == nil {
			t.Errorf("%s must error", tc.name)
		}
	}
}
