// SPDX-License-Identifier: GPL-2.0-only

package geom2d

import (
	"math"
	"testing"
)

// TestMedialAxisSlotIsCentreline checks the medial axis of a long thin slot runs along its
// centreline. A rectangle's true skeleton is the central line plus 45° spurs to each corner, so the
// centreline assertion is made only over the central span, away from the end spurs; the points must
// still run the length of the slot.
func TestMedialAxisSlotIsCentreline(t *testing.T) {
	slot := Polygon{{0, 0}, {40, 0}, {40, 10}, {0, 10}} // centreline y = 5, half-height 5
	pts := MedialAxisPoints(slot, 1)
	if len(pts) < 10 {
		t.Fatalf("expected a run of centreline points, got %d", len(pts))
	}
	minX, maxX := math.Inf(1), math.Inf(-1)
	for _, p := range pts {
		if p.X > 6 && p.X < 34 && math.Abs(p.Y-5) > 1.0 { // central span: must be on the centreline
			t.Errorf("central medial point %v strays from the y=5 centreline", p)
		}
		minX, maxX = math.Min(minX, p.X), math.Max(maxX, p.X)
	}
	if maxX-minX < 20 { // the spine should run a good length of the 40mm slot
		t.Errorf("centreline spans only %g of the slot length", maxX-minX)
	}
}

// TestMedialAxisSquareReachesCentre checks a square's medial axis (its diagonals) is captured by the
// equidistant test — including the centre, where all four walls are equidistant — not missed the way
// an axis-aligned ridge test would miss the diagonals.
func TestMedialAxisSquareReachesCentre(t *testing.T) {
	sq := Polygon{{0, 0}, {40, 0}, {40, 40}, {0, 40}}
	pts := MedialAxisPoints(sq, 2)
	centre, diagonal := false, false
	for _, p := range pts {
		if math.Hypot(p.X-20, p.Y-20) < 2 {
			centre = true
		}
		// a point well off the axes but on a diagonal (|x-20| ~ |y-20|, away from centre)
		if math.Abs(math.Abs(p.X-20)-math.Abs(p.Y-20)) < 2 && math.Hypot(p.X-20, p.Y-20) > 8 {
			diagonal = true
		}
	}
	if !centre {
		t.Error("the square's centre should be a medial point (equidistant from all walls)")
	}
	if !diagonal {
		t.Error("the square's diagonal spokes should be captured (the point of the equidistant test)")
	}
}

// TestMedialAxisDegenerate returns nothing for bad input.
func TestMedialAxisDegenerate(t *testing.T) {
	if pts := MedialAxisPoints(Polygon{{0, 0}, {1, 1}}, 1); pts != nil {
		t.Errorf("a sub-triangle has no medial axis, got %d points", len(pts))
	}
	if pts := MedialAxisPoints(Polygon{{0, 0}, {10, 0}, {10, 10}, {0, 10}}, 0); pts != nil {
		t.Errorf("a non-positive spacing returns nothing, got %d points", len(pts))
	}
}
