// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/clipper"
)

func TestCircle2CircleIntersect(t *testing.T) {
	// Two r=100 circles centred 100 apart on the x-axis (d=100 < r is false: d>=radius). The
	// solver's guard rejects d>=radius, so this returns ok=false.
	if _, _, ok := circle2CircleIntersect(clipper.IntPoint{X: 0, Y: 0}, clipper.IntPoint{X: 100, Y: 0}, 100); ok {
		t.Fatal("centres radius-or-more apart should report no intersection (solver guard)")
	}
	// Centres 50 apart (<100): two symmetric intersections, mirror images across the x-axis.
	a, b, ok := circle2CircleIntersect(clipper.IntPoint{X: 0, Y: 0}, clipper.IntPoint{X: 50, Y: 0}, 100)
	if !ok {
		t.Fatal("close centres should intersect")
	}
	if math.Abs(a.X-b.X) > 1e-9 || math.Abs(a.Y+b.Y) > 1e-9 {
		t.Fatalf("intersections should mirror across y: %v %v", a, b)
	}
	if math.Abs(a.X-25) > 1e-9 { // both lie on the perpendicular bisector x=25
		t.Fatalf("intersection x = %g, want 25 (bisector)", a.X)
	}
}

func TestLine2CircleIntersect(t *testing.T) {
	c := clipper.IntPoint{X: 0, Y: 0}
	// Horizontal segment across the circle: crosses at x=±100.
	got := line2CircleIntersect(c, 100, DoublePoint{X: -200, Y: 0}, DoublePoint{X: 200, Y: 0}, true)
	if len(got) != 2 {
		t.Fatalf("a chord through the centre should give 2 crossings, got %d", len(got))
	}
	if math.Abs(got[0].X+100) > 1e-6 || math.Abs(got[1].X-100) > 1e-6 {
		t.Fatalf("crossings = %v, want x=-100 then +100 (nearer p1 first)", got)
	}
	// A segment entirely outside the circle: no crossings.
	if out := line2CircleIntersect(c, 100, DoublePoint{X: 200, Y: 200}, DoublePoint{X: 300, Y: 300}, true); out != nil {
		t.Fatalf("segment clear of the circle should give no crossings, got %v", out)
	}
	// Unclamped: a short segment that does not reach the circle still yields the line solutions.
	if out := line2CircleIntersect(c, 100, DoublePoint{X: 0, Y: 0}, DoublePoint{X: 1, Y: 0}, false); len(out) != 2 {
		t.Fatalf("unclamped line should give 2 solutions, got %d", len(out))
	}
}

func TestCompute2DPolygonCentroid(t *testing.T) {
	// Centroid of a square (0,0)-(100,100) is its centre (50,50).
	c := compute2DPolygonCentroid(clipper.Path{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 100}, {X: 0, Y: 100}})
	if c.X != 50 || c.Y != 50 {
		t.Fatalf("square centroid = %v, want (50,50)", c)
	}
}
