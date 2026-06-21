// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import (
	"testing"

	"oblikovati.org/cam/bridge/clipper"
)

func TestIsPointWithinCutRegion(t *testing.T) {
	outer := clipper.Path{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 100}, {X: 0, Y: 100}}
	if !isPointWithinCutRegion(clipper.Paths{outer}, clipper.IntPoint{X: 50, Y: 50}) {
		t.Fatal("a point inside the boundary should be in the cut region")
	}
	if isPointWithinCutRegion(clipper.Paths{outer}, clipper.IntPoint{X: 150, Y: 50}) {
		t.Fatal("a point outside the boundary should not be in the cut region")
	}
	// With a hole (even-odd): a point inside the hole is in an odd nesting → outside the region.
	hole := clipper.Path{{X: 40, Y: 40}, {X: 60, Y: 40}, {X: 60, Y: 60}, {X: 40, Y: 60}}
	if isPointWithinCutRegion(clipper.Paths{outer, hole}, clipper.IntPoint{X: 50, Y: 50}) {
		t.Fatal("a point inside a hole should not be in the cut region")
	}
	if !isPointWithinCutRegion(clipper.Paths{outer, hole}, clipper.IntPoint{X: 10, Y: 10}) {
		t.Fatal("a point in the boundary but outside the hole should be in the region")
	}
}

func TestConventionalFraction(t *testing.T) {
	if got := conventionalFraction(0, 5); got != 0 {
		t.Fatalf("zero area should give fraction 0, got %g", got)
	}
	if got := conventionalFraction(10, 4); got != 0.4 {
		t.Fatalf("conventionalFraction = %g, want 0.4", got)
	}
}
