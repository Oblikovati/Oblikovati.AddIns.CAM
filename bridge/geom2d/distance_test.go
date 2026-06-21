// SPDX-License-Identifier: GPL-2.0-only

package geom2d

import (
	"math"
	"testing"
)

// TestDistanceToBoundary covers a point inside a square, on an edge, and at a corner.
func TestDistanceToBoundary(t *testing.T) {
	sq := Polygon{{0, 0}, {10, 0}, {10, 10}, {0, 10}}
	cases := []struct {
		name string
		pt   Point2
		want float64
	}{
		{"centre", Point2{5, 5}, 5},        // 5 from every wall
		{"near one wall", Point2{2, 5}, 2}, // 2 from the left wall
		{"on an edge", Point2{5, 0}, 0},    // on the bottom edge
		{"at a corner", Point2{0, 0}, 0},   // on a vertex
		{"outside", Point2{-3, 5}, 3},      // 3 outside the left wall
		{"near a corner", Point2{1, 1}, 1}, // 1 from the bottom (and left) wall
	}
	for _, c := range cases {
		if got := DistanceToBoundary(c.pt, sq); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: DistanceToBoundary(%v) = %g, want %g", c.name, c.pt, got, c.want)
		}
	}
}

// TestDistanceToBoundaryDegenerate returns +Inf for a polygon with no edges.
func TestDistanceToBoundaryDegenerate(t *testing.T) {
	if d := DistanceToBoundary(Point2{0, 0}, Polygon{{1, 1}}); !math.IsInf(d, 1) {
		t.Errorf("a single-point polygon has no boundary, want +Inf, got %g", d)
	}
}

// TestMaxInscribedCircle checks the largest inscribed circle of a square is centred with radius
// half its side, and of a wide rectangle is centred with radius half its short side.
func TestMaxInscribedCircle(t *testing.T) {
	c, r := MaxInscribedCircle(Polygon{{0, 0}, {20, 0}, {20, 20}, {0, 20}})
	if math.Abs(c.X-10) > 0.1 || math.Abs(c.Y-10) > 0.1 || math.Abs(r-10) > 0.1 {
		t.Errorf("square 20: centre %v r %g, want ~(10,10) r~10", c, r)
	}
	// A 40×10 slot: the biggest circle has radius 5 (half the short side), centred on the spine.
	_, r2 := MaxInscribedCircle(Polygon{{0, 0}, {40, 0}, {40, 10}, {0, 10}})
	if math.Abs(r2-5) > 0.1 {
		t.Errorf("40×10 slot inscribed radius = %g, want ~5", r2)
	}
}

// TestMaxInscribedCircleDegenerate returns a zero circle for a sub-triangle.
func TestMaxInscribedCircleDegenerate(t *testing.T) {
	if _, r := MaxInscribedCircle(Polygon{{0, 0}, {1, 1}}); r != 0 {
		t.Errorf("a degenerate polygon should give a zero radius, got %g", r)
	}
}

// TestDistanceToSegmentClampsToEndpoints checks the foot of the perpendicular is clamped to the
// segment: beyond an end, the distance is to that endpoint.
func TestDistanceToSegmentClampsToEndpoints(t *testing.T) {
	a, b := Point2{0, 0}, Point2{10, 0}
	if d := distanceToSegment(Point2{-3, 4}, a, b); math.Abs(d-5) > 1e-9 { // past A → dist to A = 5
		t.Errorf("clamp to start: got %g, want 5", d)
	}
	if d := distanceToSegment(Point2{5, 3}, a, b); math.Abs(d-3) > 1e-9 { // perpendicular within
		t.Errorf("perpendicular: got %g, want 3", d)
	}
	if d := distanceToSegment(Point2{1, 1}, a, a); math.Abs(d-math.Sqrt2) > 1e-9 { // degenerate segment
		t.Errorf("degenerate segment: got %g, want sqrt2", d)
	}
}
