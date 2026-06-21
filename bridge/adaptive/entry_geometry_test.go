// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import (
	"testing"

	"oblikovati.org/cam/bridge/clipper"
)

func TestPathsBoundsAndHasPoints(t *testing.T) {
	bb := pathsBounds(clipper.Paths{{{X: 5, Y: 5}, {X: -3, Y: 10}}, {{X: 8, Y: -2}}})
	if bb != (boundBox{minX: -3, maxX: 8, minY: -2, maxY: 10}) {
		t.Fatalf("pathsBounds = %+v, want (-3,-2)-(8,10)", bb)
	}
	if !hasPoints(clipper.Paths{{{X: 0, Y: 0}}}) {
		t.Fatal("a path with a vertex should report points")
	}
	if hasPoints(clipper.Paths{{}, nil}) {
		t.Fatal("only-empty paths should report no points")
	}
}

func TestQuadrantRect(t *testing.T) {
	// Box (0,0)-(100,80): the bottom-left quadrant rectangle has corners on the box's lower-left.
	r := quadrantRect(boundBox{minX: 0, maxX: 100, minY: 0, maxY: 80})
	want := clipper.Path{{X: 0, Y: 80}, {X: 0, Y: 40}, {X: 50, Y: 40}, {X: 50, Y: 80}}
	if len(r) != 4 {
		t.Fatalf("quadrantRect has %d corners, want 4", len(r))
	}
	for i := range want {
		if r[i] != want[i] {
			t.Fatalf("quadrantRect = %v, want %v", r, want)
		}
	}
}

func TestPickDeepestCentroid(t *testing.T) {
	// Outer boundary is a 100x100 square (index 0); the deepest offset is a small inner square,
	// whose centroid (50,50) lies inside the boundary → accepted.
	boundary := clipper.Path{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 100}, {X: 0, Y: 100}}
	lastValid := clipper.Paths{{{X: 40, Y: 40}, {X: 60, Y: 40}, {X: 60, Y: 60}, {X: 40, Y: 60}}}
	entry, ok := pickDeepestCentroid(lastValid, clipper.Paths{boundary})
	if !ok || entry != (clipper.IntPoint{X: 50, Y: 50}) {
		t.Fatalf("centroid = %v ok=%v, want (50,50) accepted", entry, ok)
	}
}

func TestPickDeepestCentroidRejectsInHole(t *testing.T) {
	// checkPaths = boundary (index 0) + a hole (index 1) covering the candidate centroid; the
	// candidate must be rejected because it lands in the hole.
	boundary := clipper.Path{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 100}, {X: 0, Y: 100}}
	hole := clipper.Path{{X: 40, Y: 40}, {X: 60, Y: 40}, {X: 60, Y: 60}, {X: 40, Y: 60}}
	lastValid := clipper.Paths{{{X: 45, Y: 45}, {X: 55, Y: 45}, {X: 55, Y: 55}, {X: 45, Y: 55}}}
	if _, ok := pickDeepestCentroid(lastValid, clipper.Paths{boundary, hole}); ok {
		t.Fatal("a centroid landing inside a hole should be rejected")
	}
}

func TestPickDeepestCentroidNoOffset(t *testing.T) {
	if _, ok := pickDeepestCentroid(clipper.Paths{{}}, nil); ok {
		t.Fatal("no non-empty offset means no entry point")
	}
}

func TestLargestHelixThatFits(t *testing.T) {
	// A monotone fit predicate (everything up to 700 fits): the search must return exactly 700.
	calls := 0
	got, err := largestHelixThatFits(150, 1200, func(r int64) (bool, error) {
		calls++
		return r <= 700, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != 700 {
		t.Fatalf("largest fitting radius = %d, want 700", got)
	}
	if calls == 0 {
		t.Fatal("fit predicate was never called")
	}
}
