// SPDX-License-Identifier: GPL-2.0-only

package geom2d

import "testing"

// ringPath returns a closed square loop (first point repeated at the end) of half-size h about
// the origin, as a clip path.
func ringPath(h float64) []Point2 {
	return []Point2{{-h, -h}, {h, -h}, {h, h}, {-h, h}, {-h, -h}}
}

// TestClipOutsideFullyOutside keeps a ring untouched when it never enters the keepout.
func TestClipOutsideFullyOutside(t *testing.T) {
	island := Polygon{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} // small central island
	runs := ClipOutside(ringPath(10), island)             // a big ring well clear of it
	if len(runs) != 1 || len(runs[0]) != 5 {
		t.Fatalf("a ring clear of the island should pass through whole, got %d runs %v", len(runs), runs)
	}
}

// TestClipOutsideFullyInside drops a ring entirely inside the keepout.
func TestClipOutsideFullyInside(t *testing.T) {
	island := Polygon{{-10, -10}, {10, -10}, {10, 10}, {-10, 10}} // big island
	runs := ClipOutside(ringPath(2), island)                      // a small ring inside it
	if len(runs) != 0 {
		t.Errorf("a ring inside the island should be fully removed, got %v", runs)
	}
}

// TestClipOutsideStraddles splits a path that crosses the island into outside runs, none of whose
// points lie inside the island.
func TestClipOutsideStraddles(t *testing.T) {
	island := Polygon{{-2, -2}, {2, -2}, {2, 2}, {-2, 2}}
	// a horizontal line crossing the island left-to-right at y=0.
	path := []Point2{{-10, 0}, {10, 0}}
	runs := ClipOutside(path, island)
	if len(runs) != 2 {
		t.Fatalf("a line through the island should split into 2 outside runs, got %d: %v", len(runs), runs)
	}
	// the first run spans from the left end to the −X wall; the second from the +X wall to the
	// right end. The interior of each run must clear the island (test a point just inside).
	if runs[0][0] != (Point2{-10, 0}) || !approxP(runs[0][len(runs[0])-1], Point2{-2, 0}) {
		t.Errorf("first run should span -10..-2, got %v", runs[0])
	}
	if !approxP(runs[1][0], Point2{2, 0}) || runs[1][len(runs[1])-1] != (Point2{10, 0}) {
		t.Errorf("second run should span 2..10, got %v", runs[1])
	}
	if island.Contains(Point2{-6, 0}) || island.Contains(Point2{6, 0}) {
		t.Error("the kept run interiors must lie outside the island")
	}
}

// approxP reports whether two points coincide within a small tolerance.
func approxP(a, b Point2) bool { return dist(a, b) < 1e-6 }

// TestClipInsideKeepsInteriorSpan keeps only the portion of a line that lies inside the region.
func TestClipInsideKeepsInteriorSpan(t *testing.T) {
	region := Polygon{{-5, -5}, {5, -5}, {5, 5}, {-5, 5}}
	path := []Point2{{-10, 0}, {10, 0}} // crosses the region left-to-right
	runs := ClipInside(path, region)
	if len(runs) != 1 {
		t.Fatalf("a line through the region should yield one inside run, got %d: %v", len(runs), runs)
	}
	if !approxP(runs[0][0], Point2{-5, 0}) || !approxP(runs[0][len(runs[0])-1], Point2{5, 0}) {
		t.Errorf("inside run should span -5..5, got %v", runs[0])
	}
}

// TestClipInsideFullyOutside drops a line that never enters the region.
func TestClipInsideFullyOutside(t *testing.T) {
	region := Polygon{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}}
	if runs := ClipInside([]Point2{{-10, 5}, {10, 5}}, region); len(runs) != 0 {
		t.Errorf("a line clear of the region should yield no runs, got %v", runs)
	}
}

// TestClipInsideDegenerateRegion returns no runs for a <3-point region (nothing is inside it).
func TestClipInsideDegenerateRegion(t *testing.T) {
	if runs := ClipInside(ringPath(5), Polygon{{0, 0}, {1, 1}}); runs != nil {
		t.Errorf("a degenerate region should yield no runs, got %v", runs)
	}
}

// TestClipOutsideDegenerateKeepout returns the path unchanged for a <3-point keepout.
func TestClipOutsideDegenerateKeepout(t *testing.T) {
	path := ringPath(5)
	if runs := ClipOutside(path, Polygon{{0, 0}, {1, 1}}); len(runs) != 1 || len(runs[0]) != len(path) {
		t.Errorf("a degenerate keepout should pass the path through, got %v", runs)
	}
}
