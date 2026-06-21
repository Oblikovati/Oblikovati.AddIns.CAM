// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

package voronoi

import (
	"math"
	"testing"
)

// TestMedialAxisSquareClearance pins the clearance: on a 10mm square, the medial axis meets at the
// centre, where the largest inscribed circle has radius 5mm (the half-width). So the medial vertex at
// the centre must carry a clearance of 5·scale, and twin edges are emitted once.
func TestMedialAxisSquareClearance(t *testing.T) {
	const s = 1000.0
	square := closedPolygon([][2]float64{{0, 0}, {10, 0}, {10, 10}, {0, 10}}, s)

	edges, err := MedialAxis(square)
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) == 0 {
		t.Fatal("the square should have a medial axis")
	}

	found := false
	for _, e := range edges {
		for _, v := range []MedialVertex{e.A, e.B} {
			if math.Hypot(v.X-5*s, v.Y-5*s) <= 1e-6*s {
				found = true
				if math.Abs(v.Clearance-5*s) > 1e-3*s {
					t.Errorf("centre clearance = %.3f, want %.3f (half the 10mm square)", v.Clearance/s, 5.0)
				}
			}
		}
	}
	if !found {
		t.Fatal("expected a medial vertex at the square centre")
	}
}

// TestMedialAxisCombClearances runs the medial axis on the comb polygon, whose corners produce
// point-source (parabolic) medial edges as well as segment-source ones, so both clearance paths are
// exercised. Every clearance must be finite and non-negative.
func TestMedialAxisCombClearances(t *testing.T) {
	const s = 1e6
	comb := closedPolygon([][2]float64{
		{0, 0}, {3.5, 0}, {3.5, 1}, {1, 1}, {1, 2}, {2.5, 2},
		{2.5, 3}, {1, 3}, {1, 4}, {3.5, 4}, {3.5, 5}, {0, 5},
	}, s)
	edges, err := MedialAxis(comb)
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) == 0 {
		t.Fatal("the comb polygon should have a medial axis")
	}
	for _, e := range edges {
		for _, v := range []MedialVertex{e.A, e.B} {
			if v.Clearance < -1e-6 || math.IsNaN(v.Clearance) || math.IsInf(v.Clearance, 0) {
				t.Errorf("bad clearance %v at (%.1f,%.1f)", v.Clearance, v.X, v.Y)
			}
		}
	}
}

// TestBuildPointsAndEmpty covers the points-input path and the empty diagram: a point site mixed with
// the square still builds, and a build with no input yields no edges.
func TestBuildPointsAndEmpty(t *testing.T) {
	const s = 1000.0
	square := closedPolygon([][2]float64{{0, 0}, {10, 0}, {10, 10}, {0, 10}}, s)
	d, err := Build([]Point{{X: 5 * s, Y: 5 * s}}, square)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Edges) == 0 {
		t.Error("a point inside the square should still produce a diagram")
	}
	empty, err := Build(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.Edges) != 0 {
		t.Errorf("an empty input should give no edges, got %d", len(empty.Edges))
	}
}

// TestMedialAxisClearanceShrinksToWall checks the clearance falls off toward the walls: every medial
// vertex's clearance is at most the centre clearance (5mm·s) and never negative.
func TestMedialAxisClearanceShrinksToWall(t *testing.T) {
	const s = 1000.0
	square := closedPolygon([][2]float64{{0, 0}, {10, 0}, {10, 10}, {0, 10}}, s)
	edges, err := MedialAxis(square)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range edges {
		for _, v := range []MedialVertex{e.A, e.B} {
			if v.Clearance < -1e-6 || v.Clearance > 5*s+1e-3*s {
				t.Errorf("clearance %.3f out of range [0, 5mm] at (%.1f,%.1f)", v.Clearance/s, v.X/s, v.Y/s)
			}
		}
	}
}
