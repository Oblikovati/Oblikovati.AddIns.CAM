// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

package voronoi

import (
	"math"
	"testing"
)

// closedPolygon builds the segments of a closed polygon from its vertices (scaled by s), the way the
// V-carve op feeds a profile to the diagram.
func closedPolygon(pts [][2]float64, s float64) []Segment {
	segs := make([]Segment, len(pts))
	for i := range pts {
		a := pts[i]
		b := pts[(i+1)%len(pts)]
		segs[i] = Segment{
			A: Point{X: int64(a[0] * s), Y: int64(a[1] * s)},
			B: Point{X: int64(b[0] * s), Y: int64(b[1] * s)},
		}
	}
	return segs
}

// hasVertexNear reports whether any edge endpoint of the diagram lands within tol of (x,y).
func hasVertexNear(d Diagram, x, y, tol float64) bool {
	for _, e := range d.Edges {
		for _, v := range []Vertex{e.V0, e.V1} {
			if v.Valid && math.Hypot(v.X-x, v.Y-y) <= tol {
				return true
			}
		}
	}
	return false
}

// TestBuildSquareMedialAxis is the clean correctness oracle: the segment Voronoi of a square's four
// edges has its medial axis meet at the square's centre, equidistant from all four edges. So a
// Voronoi vertex must land exactly at the centre.
func TestBuildSquareMedialAxis(t *testing.T) {
	const s = 1000.0 // scale: 10mm square → 10000 units, well within int32
	square := closedPolygon([][2]float64{{0, 0}, {10, 0}, {10, 10}, {0, 10}}, s)

	d, err := Build(nil, square)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Edges) == 0 {
		t.Fatal("the square's Voronoi diagram should have edges")
	}
	if len(d.Edges)%2 != 0 {
		t.Errorf("Voronoi edges come in twin pairs, got an odd count %d", len(d.Edges))
	}
	// The centre (5,5)·s is equidistant from all four walls — a medial vertex sits exactly there.
	if !hasVertexNear(d, 5*s, 5*s, 1e-6*s) {
		t.Errorf("expected a medial vertex at the square centre (%g,%g)", 5*s, 5*s)
	}
	// The medial axis is made of primary edges.
	primary := 0
	for _, e := range d.Edges {
		if e.IsPrimary && e.V0.Valid && e.V1.Valid {
			primary++
		}
	}
	if primary == 0 {
		t.Error("the square medial axis should have finite primary edges")
	}
}

// TestBuildPolygonDiagram mirrors the upstream TestPathVoronoi setup (a 12-vertex comb polygon fed as
// segments) and checks the engine produces a non-trivial medial structure: finite primary edges, some
// of them parabolic (between a segment and one of its endpoints) and some linear.
func TestBuildPolygonDiagram(t *testing.T) {
	const s = 1e6
	comb := closedPolygon([][2]float64{
		{0, 0}, {3.5, 0}, {3.5, 1}, {1, 1}, {1, 2}, {2.5, 2},
		{2.5, 3}, {1, 3}, {1, 4}, {3.5, 4}, {3.5, 5}, {0, 5},
	}, s)

	d, err := Build(nil, comb)
	if err != nil {
		t.Fatal(err)
	}
	finitePrimary, linear, curved := 0, 0, 0
	for _, e := range d.Edges {
		if !e.IsPrimary || !e.V0.Valid || !e.V1.Valid {
			continue
		}
		finitePrimary++
		if e.IsLinear {
			linear++
		} else {
			curved++
		}
	}
	if finitePrimary == 0 {
		t.Fatal("the comb polygon should yield finite primary medial edges")
	}
	if linear == 0 {
		t.Error("expected linear medial edges (segment-vs-segment bisectors)")
	}
	if curved == 0 {
		t.Error("expected parabolic medial edges (segment-vs-endpoint bisectors)")
	}
}
