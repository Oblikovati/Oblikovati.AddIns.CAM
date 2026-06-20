// SPDX-License-Identifier: GPL-2.0-only

package geom2d

import (
	"math"
	"testing"
)

// square returns a CCW axis-aligned square [0,s]×[0,s].
func square(s float64) Polygon {
	return Polygon{{0, 0}, {s, 0}, {s, s}, {0, s}}
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

// TestPolygonMetrics covers area, winding, perimeter, and centroid.
func TestPolygonMetrics(t *testing.T) {
	sq := square(10)
	if !approx(sq.Area(), 100) {
		t.Errorf("area = %g, want 100", sq.Area())
	}
	if !sq.IsCCW() {
		t.Error("square should be CCW")
	}
	if !approx(sq.Perimeter(), 40) {
		t.Errorf("perimeter = %g, want 40", sq.Perimeter())
	}
	c := sq.Centroid()
	if !approx(c.X, 5) || !approx(c.Y, 5) {
		t.Errorf("centroid = %+v, want (5,5)", c)
	}
}

// TestEnsureCCW flips a clockwise polygon and leaves a CCW one alone.
func TestEnsureCCW(t *testing.T) {
	cw := square(10).Reversed()
	if cw.IsCCW() {
		t.Fatal("reversed square should be CW")
	}
	if !cw.EnsureCCW().IsCCW() {
		t.Error("EnsureCCW should produce a CCW polygon")
	}
	if !square(10).EnsureCCW().IsCCW() {
		t.Error("EnsureCCW on a CCW polygon must keep it CCW")
	}
}

// TestContains covers inside, outside, and a degenerate polygon.
func TestContains(t *testing.T) {
	sq := square(10)
	if !sq.Contains(Point2{5, 5}) {
		t.Error("centre should be inside")
	}
	if sq.Contains(Point2{15, 5}) {
		t.Error("point outside should not be inside")
	}
	if (Polygon{{0, 0}, {1, 1}}).Contains(Point2{0, 0}) {
		t.Error("degenerate polygon contains nothing")
	}
}

// TestOffsetOutward grows a square: a 10×10 square offset by +2 becomes a 14×14 square.
func TestOffsetOutward(t *testing.T) {
	out, ok := Offset(square(10), 2)
	if !ok {
		t.Fatal("outward offset should not collapse")
	}
	if !approx(out.Area(), 14*14) {
		t.Errorf("offset area = %g, want 196", out.Area())
	}
	// Corner (0,0) moves to (-2,-2).
	if !approx(out[0].X, -2) || !approx(out[0].Y, -2) {
		t.Errorf("corner = %+v, want (-2,-2)", out[0])
	}
}

// TestOffsetInward shrinks a square and detects collapse past the centre.
func TestOffsetInward(t *testing.T) {
	in, ok := Offset(square(10), -3)
	if !ok {
		t.Fatal("inward offset by 3 should survive")
	}
	if !approx(in.Area(), 4*4) {
		t.Errorf("inward area = %g, want 16", in.Area())
	}
	if _, ok := Offset(square(10), -5); ok {
		t.Error("inward offset by half the side should collapse")
	}
	if _, ok := Offset(square(10), -8); ok {
		t.Error("inward offset past the centre should collapse")
	}
}

// TestOffsetDegenerate rejects sub-triangle input.
func TestOffsetDegenerate(t *testing.T) {
	if _, ok := Offset(Polygon{{0, 0}, {1, 0}}, 1); ok {
		t.Error("a 2-point polygon cannot be offset")
	}
}

// TestCentroidDegenerate falls back to the vertex average for a zero-area (collinear) loop
// and returns the origin for an empty polygon.
func TestCentroidDegenerate(t *testing.T) {
	collinear := Polygon{{0, 0}, {2, 0}, {4, 0}}
	c := collinear.Centroid()
	if !approx(c.X, 2) || !approx(c.Y, 0) {
		t.Errorf("collinear centroid = %+v, want vertex average (2,0)", c)
	}
	if e := (Polygon{}).Centroid(); e != (Point2{}) {
		t.Errorf("empty centroid = %+v, want origin", e)
	}
}

// TestOffsetCollinearVertex offsets a square with an extra mid-edge (collinear) vertex,
// exercising the parallel-edge averaged-normal fallback in the corner solver.
func TestOffsetCollinearVertex(t *testing.T) {
	withMid := Polygon{{0, 0}, {5, 0}, {10, 0}, {10, 10}, {0, 10}} // (5,0) is collinear
	out, ok := Offset(withMid, 2)
	if !ok {
		t.Fatal("offset of a polygon with a collinear vertex should succeed")
	}
	if out.Area() <= withMid.Area() {
		t.Errorf("outward offset area %g should exceed original %g", out.Area(), withMid.Area())
	}
}

// TestOffsetConcaveL offsets an L-shape inward and checks the area shrinks but survives (the
// reflex corner is handled by the miter join).
func TestOffsetConcaveL(t *testing.T) {
	l := Polygon{{0, 0}, {10, 0}, {10, 4}, {4, 4}, {4, 10}, {0, 10}}
	in, ok := Offset(l, -1)
	if !ok {
		t.Fatal("L-shape inward offset by 1 should survive")
	}
	if in.Area() >= l.Area() {
		t.Errorf("inward offset area %g should be < original %g", in.Area(), l.Area())
	}
}
