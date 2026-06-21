// SPDX-License-Identifier: GPL-2.0-only

package voronoi

import "math"

// MedialVertex is a point on the medial axis with its clearance — the radius of the largest circle
// centred there that fits inside the shape (the distance to the nearest wall). Clearance sets the
// V-carve depth: the deeper the clearance, the deeper the V-bit rides. Coordinates and clearance are
// in the scaled plane (divide by the scale used to build the diagram for millimetres).
type MedialVertex struct {
	X, Y      float64
	Clearance float64
}

// MedialEdge is one finite primary Voronoi edge — a piece of the medial axis — with the clearance at
// each end.
type MedialEdge struct {
	A, B MedialVertex
}

// MedialAxis builds the segment Voronoi diagram of the boundary segments and returns its finite
// primary edges as medial edges carrying the clearance at each endpoint. Twin edges are emitted once.
// The clearance is measured the way the reference workbench does it: to a point source (a polygon
// vertex) it is the direct distance, to a segment source it is the perpendicular distance to that
// edge's supporting line.
func MedialAxis(segments []Segment) ([]MedialEdge, error) {
	d, err := Build(nil, segments)
	if err != nil {
		return nil, err
	}
	var out []MedialEdge
	seen := make(map[[4]int64]bool)
	for _, e := range d.Edges {
		if !e.IsPrimary || !e.V0.Valid || !e.V1.Valid {
			continue
		}
		key := twinKey(e)
		if seen[key] {
			continue // the reverse twin of this undirected edge was already emitted
		}
		seen[key] = true
		out = append(out, MedialEdge{
			A: MedialVertex{X: e.V0.X, Y: e.V0.Y, Clearance: clearanceAt(e.V0, e, segments)},
			B: MedialVertex{X: e.V1.X, Y: e.V1.Y, Clearance: clearanceAt(e.V1, e, segments)},
		})
	}
	return out, nil
}

// twinKey is the canonical key of an undirected edge: its two endpoints rounded to the scaled-integer
// grid and ordered, so an edge and its reverse twin map to the same key.
func twinKey(e Edge) [4]int64 {
	a := [2]int64{int64(math.Round(e.V0.X)), int64(math.Round(e.V0.Y))}
	b := [2]int64{int64(math.Round(e.V1.X)), int64(math.Round(e.V1.Y))}
	if a[0] > b[0] || (a[0] == b[0] && a[1] > b[1]) {
		a, b = b, a
	}
	return [4]int64{a[0], a[1], b[0], b[1]}
}

// clearanceAt measures the clearance at vertex v of edge e: to the cell's site if it is a point, else
// the twin cell's site if that is a point, else the perpendicular distance to the cell's segment.
// Mirrors the reference workbench's retrieveDistances dispatch.
func clearanceAt(v Vertex, e Edge, segments []Segment) float64 {
	if e.Cell.ContainsPoint() {
		px, py := sourcePoint(e.Cell, segments)
		return math.Hypot(v.X-px, v.Y-py)
	}
	if e.Twin.ContainsPoint() {
		px, py := sourcePoint(e.Twin, segments)
		return math.Hypot(v.X-px, v.Y-py)
	}
	return perpDistanceToLine(v, segments[e.Cell.Index])
}

// sourcePoint returns the point site of a point cell. With only segments inserted, a point cell is a
// segment endpoint: the low end for SEGMENT_START_POINT, the high end for SEGMENT_END_POINT (low/high
// ordered lexicographically, as Boost.Polygon does).
func sourcePoint(c CellSource, segments []Segment) (float64, float64) {
	s := segments[c.Index]
	lowX, lowY, highX, highY := segLowHigh(s)
	if c.Category == SourceSegmentEnd {
		return highX, highY
	}
	return lowX, lowY
}

// segLowHigh returns the lexicographically lower and higher endpoints of the segment.
func segLowHigh(s Segment) (lowX, lowY, highX, highY float64) {
	ax, ay := float64(s.A.X), float64(s.A.Y)
	bx, by := float64(s.B.X), float64(s.B.Y)
	if ax < bx || (ax == bx && ay < by) {
		return ax, ay, bx, by
	}
	return bx, by, ax, ay
}

// perpDistanceToLine is the perpendicular distance from v to the infinite line through the segment
// (the orthogonal-projection distance the reference workbench uses for a segment source).
func perpDistanceToLine(v Vertex, s Segment) float64 {
	ax, ay := float64(s.A.X), float64(s.A.Y)
	bx, by := float64(s.B.X), float64(s.B.Y)
	dx, dy := bx-ax, by-ay
	length := math.Hypot(dx, dy)
	if length == 0 {
		return math.Hypot(v.X-ax, v.Y-ay)
	}
	return math.Abs((v.X-ax)*dy-(v.Y-ay)*dx) / length
}
