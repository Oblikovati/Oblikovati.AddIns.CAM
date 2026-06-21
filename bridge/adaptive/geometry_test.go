// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/clipper"
)

func approx(t *testing.T, got, want, tol float64, label string) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Fatalf("%s = %g, want %g (±%g)", label, got, want, tol)
	}
}

func TestDistance(t *testing.T) {
	a, b := clipper.IntPoint{X: 0, Y: 0}, clipper.IntPoint{X: 3, Y: 4}
	if got := distanceSqrd(a, b); got != 25 {
		t.Fatalf("distanceSqrd = %g, want 25", got)
	}
	approx(t, distanceBetween(a, b), 5, 1e-9, "distanceBetween")
}

func TestSetSegmentLength(t *testing.T) {
	p1 := clipper.IntPoint{X: 0, Y: 0}
	p2 := clipper.IntPoint{X: 10, Y: 0}
	if !setSegmentLength(p1, &p2, 4) {
		t.Fatal("setSegmentLength returned false for a non-zero segment")
	}
	if p2 != (clipper.IntPoint{X: 4, Y: 0}) {
		t.Fatalf("setSegmentLength moved pt2 to %v, want (4,0)", p2)
	}
	zero := clipper.IntPoint{X: 0, Y: 0}
	if setSegmentLength(p1, &zero, 4) {
		t.Fatal("setSegmentLength should return false for a zero-length segment")
	}
}

func TestRotateVec(t *testing.T) {
	got := rotateVec(DoublePoint{X: 1, Y: 0}, math.Pi/2)
	approx(t, got.X, 0, 1e-9, "rotate.X")
	approx(t, got.Y, 1, 1e-9, "rotate.Y")
}

func TestPathLength(t *testing.T) {
	p := clipper.Path{{X: 0, Y: 0}, {X: 3, Y: 0}, {X: 3, Y: 4}}
	approx(t, pathLength(p), 7, 1e-9, "pathLength")
	if pathLength(clipper.Path{{X: 1, Y: 1}}) != 0 {
		t.Fatal("single-point path should have length 0")
	}
}

func TestPointSideOfLine(t *testing.T) {
	// The upstream term is (pt.X-p1.X)(p2.Y-p1.Y) - (pt.Y-p2.Y)(p2.X-p1.X); for the x-axis
	// p1(0,0)->p2(10,0) a point ABOVE evaluates negative and one BELOW positive. Ported as-is.
	p1, p2 := clipper.IntPoint{X: 0, Y: 0}, clipper.IntPoint{X: 10, Y: 0}
	if pointSideOfLine(p1, p2, clipper.IntPoint{X: 5, Y: 5}) >= 0 {
		t.Fatal("point above the x-axis line should evaluate negative (upstream sign convention)")
	}
	if pointSideOfLine(p1, p2, clipper.IntPoint{X: 5, Y: -5}) <= 0 {
		t.Fatal("point below the x-axis line should evaluate positive (upstream sign convention)")
	}
}

func TestAngle3Points(t *testing.T) {
	// A right-angle turn: 0,0 → 1,0 → 1,1 turns by π/2.
	a := angle3Points(DoublePoint{0, 0}, DoublePoint{1, 0}, DoublePoint{1, 1})
	approx(t, a, math.Pi/2, 1e-9, "angle3Points right turn")
	// Straight ahead turns by 0.
	a = angle3Points(DoublePoint{0, 0}, DoublePoint{1, 0}, DoublePoint{2, 0})
	approx(t, a, 0, 1e-9, "angle3Points straight")
}

func TestDirectionV(t *testing.T) {
	d := directionV(clipper.IntPoint{X: 0, Y: 0}, clipper.IntPoint{X: 0, Y: 5})
	approx(t, d.X, 0, 1e-9, "dir.X")
	approx(t, d.Y, 1, 1e-9, "dir.Y")
	if z := directionV(clipper.IntPoint{X: 2, Y: 2}, clipper.IntPoint{X: 2, Y: 2}); z != (DoublePoint{}) {
		t.Fatalf("coincident points should give zero direction, got %v", z)
	}
}

func TestNormalizeV(t *testing.T) {
	v := DoublePoint{X: 3, Y: 4}
	normalizeV(&v)
	approx(t, math.Hypot(v.X, v.Y), 1, 1e-9, "normalized length")
	z := DoublePoint{}
	normalizeV(&z) // must not divide by zero
	if z != (DoublePoint{}) {
		t.Fatalf("normalizing zero should stay zero, got %v", z)
	}
}

func TestGetPathDirectionV(t *testing.T) {
	p := clipper.Path{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}}
	// Edge entering vertex 0 wraps from the last vertex (10,10)→(0,0): unit (-√2/2, -√2/2).
	d := getPathDirectionV(p, 0)
	approx(t, d.X, -math.Sqrt2/2, 1e-9, "wrap edge dir.X")
	approx(t, d.Y, -math.Sqrt2/2, 1e-9, "wrap edge dir.Y")
	// Edge entering vertex 1 is (0,0)→(10,0): +X.
	d = getPathDirectionV(p, 1)
	approx(t, d.X, 1, 1e-9, "edge dir.X")
	if getPathDirectionV(clipper.Path{{X: 0, Y: 0}}, 0) != (DoublePoint{}) {
		t.Fatal("degenerate path should give zero direction")
	}
}

func TestPointsCoincident(t *testing.T) {
	if !pointsCoincident(clipper.IntPoint{X: 5, Y: 5}, clipper.IntPoint{X: 6, Y: 4}) {
		t.Fatal("points within 1 unit per axis should be coincident")
	}
	if pointsCoincident(clipper.IntPoint{X: 5, Y: 5}, clipper.IntPoint{X: 7, Y: 5}) {
		t.Fatal("points 2 units apart should not be coincident")
	}
}

func TestTranslatePath(t *testing.T) {
	out := translatePath(clipper.Path{{X: 0, Y: 0}, {X: 1, Y: 2}}, clipper.IntPoint{X: 10, Y: 20})
	if out[0] != (clipper.IntPoint{X: 10, Y: 20}) || out[1] != (clipper.IntPoint{X: 11, Y: 22}) {
		t.Fatalf("translatePath = %v", out)
	}
}

func TestAverageDirection(t *testing.T) {
	// Two unit vectors at ±45° average to +X.
	d := averageDirection([]DoublePoint{{X: math.Sqrt2 / 2, Y: math.Sqrt2 / 2}, {X: math.Sqrt2 / 2, Y: -math.Sqrt2 / 2}})
	approx(t, d.X, 1, 1e-9, "avg dir.X")
	approx(t, d.Y, 0, 1e-9, "avg dir.Y")
	// Opposing vectors sum to zero → zero vector (no divide-by-zero).
	if z := averageDirection([]DoublePoint{{X: 1}, {X: -1}}); z != (DoublePoint{}) {
		t.Fatalf("opposing vectors should average to zero, got %v", z)
	}
	if z := averageDirection(nil); z != (DoublePoint{}) {
		t.Fatalf("empty input should average to zero, got %v", z)
	}
}

func TestAverageDV(t *testing.T) {
	approx(t, averageDV([]float64{1, 2, 3, 4}), 2.5, 1e-9, "averageDV")
	if averageDV(nil) != 0 {
		t.Fatal("averageDV of empty should be 0")
	}
}

func TestDistancePointToLineSegSquared(t *testing.T) {
	p1, p2 := clipper.IntPoint{X: 0, Y: 0}, clipper.IntPoint{X: 10, Y: 0}
	// Point above the middle: distance 5, closest (5,0), param 0.5.
	d, closest, param := distancePointToLineSegSquared(p1, p2, clipper.IntPoint{X: 5, Y: 5}, true)
	approx(t, d, 25, 1e-9, "perp dist sq")
	if closest != (clipper.IntPoint{X: 5, Y: 0}) {
		t.Fatalf("closest = %v, want (5,0)", closest)
	}
	approx(t, param, 0.5, 1e-9, "param")
	// Point beyond p2, clamped: closest is p2.
	_, closest, param = distancePointToLineSegSquared(p1, p2, clipper.IntPoint{X: 20, Y: 0}, true)
	if closest != p2 || param != 1 {
		t.Fatalf("clamped closest = %v param %g, want p2 / 1", closest, param)
	}
	// Unclamped: the projection runs past p2 (param > 1).
	if _, _, param = distancePointToLineSegSquared(p1, p2, clipper.IntPoint{X: 20, Y: 0}, false); param != 2 {
		t.Fatalf("unclamped param = %g, want 2", param)
	}
	// Zero-length segment: point-to-point distance.
	if d, _, _ = distancePointToLineSegSquared(p1, p1, clipper.IntPoint{X: 3, Y: 4}, true); d != 25 {
		t.Fatalf("zero-length seg dist sq = %g, want 25", d)
	}
}

func TestScalePaths(t *testing.T) {
	ps := clipper.Paths{{{X: 1, Y: 2}, {X: 3, Y: 4}}}
	scaleUpPaths(ps, 100)
	if ps[0][1] != (clipper.IntPoint{X: 300, Y: 400}) {
		t.Fatalf("scaleUpPaths = %v", ps)
	}
	scaleDownPaths(ps, 100)
	if ps[0][1] != (clipper.IntPoint{X: 3, Y: 4}) {
		t.Fatalf("scaleDownPaths = %v", ps)
	}
}

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if c.ToolDiameter != 5 || c.StepOverFactor != 0.2 || c.Tolerance != 0.1 {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if !c.ForceInsideOut || !c.FinishingProfile || c.KeepToolDownDistRatio != 3.0 {
		t.Fatalf("unexpected behaviour defaults: %+v", c)
	}
	if c.OpType != ClearingInside {
		t.Fatalf("default OpType = %v, want ClearingInside", c.OpType)
	}
}
