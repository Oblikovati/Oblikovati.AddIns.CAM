// SPDX-License-Identifier: GPL-2.0-only

package clipper

import "testing"

// ccwSquare is a 10x10 counter-clockwise square at the origin: signed area +100.
func ccwSquare() Path {
	return Path{{0, 0, 0}, {10, 0, 0}, {10, 10, 0}, {0, 10, 0}}
}

func TestAreaSignedByWinding(t *testing.T) {
	if got := Area(ccwSquare()); got != 100 {
		t.Fatalf("CCW square area = %g, want 100", got)
	}
	cw := ccwSquare()
	ReversePath(cw)
	if got := Area(cw); got != -100 {
		t.Fatalf("CW square area = %g, want -100", got)
	}
}

func TestAreaDegenerate(t *testing.T) {
	for _, p := range []Path{nil, {{0, 0, 0}}, {{0, 0, 0}, {10, 0, 0}}} {
		if got := Area(p); got != 0 {
			t.Fatalf("Area(%v) = %g, want 0 for a <3-vertex path", p, got)
		}
	}
}

func TestOrientationMatchesAreaSign(t *testing.T) {
	if !Orientation(ccwSquare()) {
		t.Fatal("CCW square should be positively oriented")
	}
	cw := ccwSquare()
	ReversePath(cw)
	if Orientation(cw) {
		t.Fatal("CW square should be negatively oriented")
	}
}

func TestPointInPolygon(t *testing.T) {
	sq := ccwSquare()
	cases := []struct {
		name string
		pt   IntPoint
		want int
	}{
		{"strictly inside", IntPoint{5, 5, 0}, 1},
		{"outside to the right", IntPoint{15, 5, 0}, 0},
		{"outside below", IntPoint{5, -5, 0}, 0},
		{"on the left edge", IntPoint{0, 5, 0}, -1},
		{"on the bottom edge", IntPoint{5, 0, 0}, -1},
		{"on a corner vertex", IntPoint{10, 10, 0}, -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := PointInPolygon(c.pt, sq); got != c.want {
				t.Fatalf("PointInPolygon(%v) = %d, want %d", c.pt, got, c.want)
			}
		})
	}
}

func TestPointInPolygonDegenerate(t *testing.T) {
	if got := PointInPolygon(IntPoint{0, 0, 0}, Path{{0, 0, 0}, {1, 1, 0}}); got != 0 {
		t.Fatalf("PointInPolygon on a <3-vertex path = %d, want 0", got)
	}
}

func TestReversePath(t *testing.T) {
	p := Path{{0, 0, 0}, {1, 0, 0}, {2, 0, 0}}
	ReversePath(p)
	want := Path{{2, 0, 0}, {1, 0, 0}, {0, 0, 0}}
	for i := range want {
		if p[i] != want[i] {
			t.Fatalf("ReversePath = %v, want %v", p, want)
		}
	}
}

func TestReversePaths(t *testing.T) {
	ps := Paths{{{0, 0, 0}, {1, 0, 0}}, {{0, 0, 0}, {0, 1, 0}}}
	ReversePaths(ps)
	if ps[0][0] != (IntPoint{1, 0, 0}) || ps[1][0] != (IntPoint{0, 1, 0}) {
		t.Fatalf("ReversePaths did not reverse each path: %v", ps)
	}
}

func TestCleanPolygonRemovesCollinearVertex(t *testing.T) {
	// A 100x100 square with a redundant point at (50,0) on the bottom edge: it is exactly
	// collinear, so cleaning at distance 2 must drop it, leaving the 4 corners.
	in := Path{{0, 0, 0}, {50, 0, 0}, {100, 0, 0}, {100, 100, 0}, {0, 100, 0}}
	out := CleanPolygon(in, 2)
	if len(out) != 4 {
		t.Fatalf("CleanPolygon kept %d vertices, want 4 (collinear point removed): %v", len(out), out)
	}
	if Area(out) != 10000 {
		t.Fatalf("cleaned square area = %g, want 10000 (shape preserved)", Area(out))
	}
}

func TestCleanPolygonRemovesNearDuplicate(t *testing.T) {
	// (100,1) sits 1 unit from (100,0) — within distance 2 — so it is stripped as a
	// near-duplicate, again leaving 4 corners of a ~square.
	in := Path{{0, 0, 0}, {100, 0, 0}, {100, 1, 0}, {100, 100, 0}, {0, 100, 0}}
	out := CleanPolygon(in, 2)
	if len(out) != 4 {
		t.Fatalf("CleanPolygon kept %d vertices, want 4 (near-duplicate removed): %v", len(out), out)
	}
}

func TestCleanPolygonCollapsesToEmpty(t *testing.T) {
	// A degenerate sliver collapses below 3 vertices, which Clipper reports as empty.
	if out := CleanPolygon(Path{{0, 0, 0}, {1, 0, 0}, {2, 0, 0}}, 5); len(out) != 0 {
		t.Fatalf("CleanPolygon of a collinear sliver = %v, want empty", out)
	}
	if out := CleanPolygon(nil, 2); len(out) != 0 {
		t.Fatalf("CleanPolygon(nil) = %v, want empty", out)
	}
}

func TestCleanPolygons(t *testing.T) {
	out := CleanPolygons(Paths{{{0, 0, 0}, {50, 0, 0}, {100, 0, 0}, {100, 100, 0}, {0, 100, 0}}}, 2)
	if len(out) != 1 || len(out[0]) != 4 {
		t.Fatalf("CleanPolygons = %v, want one 4-vertex path", out)
	}
}
