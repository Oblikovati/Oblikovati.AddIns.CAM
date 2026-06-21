// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/clipper"
)

func TestGetPathNestingLevel(t *testing.T) {
	outer := clipper.Path{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 100}, {X: 0, Y: 100}}
	inner := clipper.Path{{X: 20, Y: 20}, {X: 80, Y: 20}, {X: 80, Y: 80}, {X: 20, Y: 80}}
	// A point inside both polygons nests twice; inside only the outer, once.
	if got := getPathNestingLevel(clipper.Path{{X: 50, Y: 50}}, clipper.Paths{outer, inner}); got != 2 {
		t.Fatalf("nesting at centre = %d, want 2", got)
	}
	if got := getPathNestingLevel(clipper.Path{{X: 10, Y: 10}}, clipper.Paths{outer, inner}); got != 1 {
		t.Fatalf("nesting near corner = %d, want 1", got)
	}
	if got := getPathNestingLevel(clipper.Path{{X: 200, Y: 200}}, clipper.Paths{outer, inner}); got != 0 {
		t.Fatalf("nesting outside = %d, want 0", got)
	}
}

func TestIntersectionPointCrossing(t *testing.T) {
	pt, ok := intersectionPoint(
		clipper.IntPoint{X: -10, Y: 0}, clipper.IntPoint{X: 10, Y: 0},
		clipper.IntPoint{X: 0, Y: -10}, clipper.IntPoint{X: 0, Y: 10},
	)
	if !ok {
		t.Fatal("crossing segments should intersect")
	}
	if pt.X != 0 || pt.Y != 0 {
		t.Fatalf("intersection = %+v, want origin", pt)
	}
}

func TestIntersectionPointParallelAndDisjoint(t *testing.T) {
	if _, ok := intersectionPoint(
		clipper.IntPoint{X: 0, Y: 0}, clipper.IntPoint{X: 10, Y: 0},
		clipper.IntPoint{X: 0, Y: 5}, clipper.IntPoint{X: 10, Y: 5},
	); ok {
		t.Fatal("parallel segments should not intersect")
	}
	// Lines cross but not within both segments.
	if _, ok := intersectionPoint(
		clipper.IntPoint{X: 0, Y: 0}, clipper.IntPoint{X: 10, Y: 0},
		clipper.IntPoint{X: 20, Y: -10}, clipper.IntPoint{X: 20, Y: 10},
	); ok {
		t.Fatal("non-overlapping segments should not intersect")
	}
}

func TestIntersectionPointPaths(t *testing.T) {
	square := clipper.Path{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 100}, {X: 0, Y: 100}}
	if _, ok := intersectionPointPaths(clipper.Paths{square}, clipper.IntPoint{X: 50, Y: 50}, clipper.IntPoint{X: 200, Y: 50}); !ok {
		t.Fatal("a ray leaving the square should cross its boundary")
	}
	if _, ok := intersectionPointPaths(clipper.Paths{square}, clipper.IntPoint{X: 200, Y: 200}, clipper.IntPoint{X: 300, Y: 200}); ok {
		t.Fatal("a segment well outside the square should not cross it")
	}
}

func TestConnectPathsJoinsChains(t *testing.T) {
	a := clipper.Path{{X: 0, Y: 0}, {X: 10, Y: 0}}
	b := clipper.Path{{X: 10, Y: 0}, {X: 20, Y: 0}}
	out := connectPaths(clipper.Paths{a, b})
	if len(out) != 1 {
		t.Fatalf("two touching paths should join into one, got %d", len(out))
	}
	if len(out[0]) != 4 {
		t.Fatalf("joined path length = %d, want 4", len(out[0]))
	}
	if out[0][0] != (clipper.IntPoint{X: 0, Y: 0}) || out[0][3] != (clipper.IntPoint{X: 20, Y: 0}) {
		t.Fatalf("joined endpoints = %+v..%+v", out[0][0], out[0][3])
	}
}

func TestConnectPathsReversesToJoin(t *testing.T) {
	a := clipper.Path{{X: 0, Y: 0}, {X: 10, Y: 0}}
	// b shares its END with a's end, so it must be reversed to chain.
	b := clipper.Path{{X: 20, Y: 0}, {X: 10, Y: 0}}
	out := connectPaths(clipper.Paths{a, b})
	if len(out) != 1 || len(out[0]) != 4 {
		t.Fatalf("reversed join failed: %v", out)
	}
	if out[0][3] != (clipper.IntPoint{X: 20, Y: 0}) {
		t.Fatalf("end after reversed join = %+v, want {20,0}", out[0][3])
	}
	// The caller's path must not have been mutated by the internal reversal.
	if b[0] != (clipper.IntPoint{X: 20, Y: 0}) {
		t.Fatal("connectPaths must not mutate the caller's paths")
	}
}

func TestDeduplicatePaths(t *testing.T) {
	a := clipper.Path{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}}
	dup := clipper.Path{{X: 0, Y: 0}, {X: 10, Y: 0}} // all points lie on a
	uniq := clipper.Path{{X: 50, Y: 50}}
	out := deduplicatePaths(clipper.Paths{a, dup, uniq})
	if len(out) != 2 {
		t.Fatalf("deduplicated count = %d, want 2", len(out))
	}
}

func TestPopPathWithClosestPoint(t *testing.T) {
	a := clipper.Path{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10}}
	b := clipper.Path{{X: 100, Y: 100}, {X: 110, Y: 100}}
	result, remaining, ok := popPathWithClosestPoint(clipper.Paths{a, b}, clipper.IntPoint{X: 9, Y: 11}, 0)
	if !ok {
		t.Fatal("pop should succeed with a non-empty collection")
	}
	if len(remaining) != 1 || remaining[0][0] != (clipper.IntPoint{X: 100, Y: 100}) {
		t.Fatalf("remaining should be just b, got %v", remaining)
	}
	// Closest vertex to (9,11) is (10,10); the result must start there.
	if result[0] != (clipper.IntPoint{X: 10, Y: 10}) {
		t.Fatalf("result should start at the closest vertex, got %+v", result[0])
	}
}

func TestPopPathWithClosestPointEmpty(t *testing.T) {
	if _, _, ok := popPathWithClosestPoint(clipper.Paths{}, clipper.IntPoint{}, 0); ok {
		t.Fatal("pop from an empty collection should fail")
	}
}

func TestCleanPathPreservesEnds(t *testing.T) {
	// A path with a redundant collinear midpoint; ends must be preserved.
	inp := clipper.Path{{X: 0, Y: 0}, {X: 50, Y: 1}, {X: 100, Y: 0}, {X: 100, Y: 100}, {X: 0, Y: 100}}
	out := cleanPath(inp, 5)
	if out[0] != inp[0] {
		t.Fatalf("first point = %+v, want %+v", out[0], inp[0])
	}
	if out[len(out)-1] != inp[len(inp)-1] {
		t.Fatalf("last point = %+v, want %+v", out[len(out)-1], inp[len(inp)-1])
	}
}

func TestCleanPathShortInput(t *testing.T) {
	inp := clipper.Path{{X: 0, Y: 0}, {X: 10, Y: 0}}
	out := cleanPath(inp, 5)
	if len(out) != 2 || out[0] != inp[0] || out[1] != inp[1] {
		t.Fatalf("short input should pass through unchanged, got %v", out)
	}
}

func TestSmoothPathsKeepsEndsAndShortens(t *testing.T) {
	// A zig-zag line; smoothing should keep the first/last points and pull the interior straighter.
	in := clipper.Path{
		{X: 0, Y: 0}, {X: 100, Y: 200}, {X: 200, Y: 0}, {X: 300, Y: 200}, {X: 400, Y: 0},
	}
	out := smoothPaths(clipper.Paths{in}, 20, 1, 4)
	if len(out) != 1 {
		t.Fatalf("smoothPaths should return one path, got %d", len(out))
	}
	if out[0][0] != (clipper.IntPoint{X: 0, Y: 0}) {
		t.Fatalf("first point moved: %+v", out[0][0])
	}
	if out[0][len(out[0])-1] != (clipper.IntPoint{X: 400, Y: 0}) {
		t.Fatalf("last point moved: %+v", out[0][len(out[0])-1])
	}
	// The averaged interior peaks should be lower than the original 200.
	maxY := int64(0)
	for _, p := range out[0] {
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	if maxY >= 200 {
		t.Fatalf("smoothing did not reduce the peak height: maxY=%d", maxY)
	}
}

func TestSmoothPathsEmpty(t *testing.T) {
	out := smoothPaths(clipper.Paths{{}}, 20, 1, 4)
	if len(out) != 1 || len(out[0]) != 0 {
		t.Fatalf("empty path should stay empty, got %v", out)
	}
}

func TestAveragePointsSmoothsInterior(t *testing.T) {
	// The very ends are never averaged (window narrows to 0 next to them); a genuine interior
	// point (i=2 of 5) is the mean of itself and its two neighbours.
	pts := []indexedPoint{
		{0, clipper.IntPoint{X: 0, Y: 0}},
		{0, clipper.IntPoint{X: 1000, Y: 0}},
		{0, clipper.IntPoint{X: 2000, Y: 3000}}, // spike
		{0, clipper.IntPoint{X: 3000, Y: 0}},
		{0, clipper.IntPoint{X: 4000, Y: 0}},
	}
	averagePoints(pts, 1, 1)
	if pts[0].pt.Y != 0 || pts[4].pt.Y != 0 {
		t.Fatalf("ends must stay put: %+v %+v", pts[0].pt, pts[4].pt)
	}
	// i=2 averages neighbours at y=0,0 and itself at 3000 → 1000.
	if pts[2].pt.Y != int64(math.Round(3000.0/3)) {
		t.Fatalf("averaged spike = %d, want 1000", pts[2].pt.Y)
	}
}
