// SPDX-License-Identifier: GPL-2.0-only

// Package voronoi is the project's thin Go interface over the Boost.Polygon Voronoi library (the
// vendored C++ under vendor/, BSL-1.0), the segment-Voronoi engine the V-carve toolpath rides. It
// builds the diagram of a set of integer points and segments and returns its edge table; the
// medial-axis extraction and the per-vertex clearance distances are computed on top of this table in
// pure Go (the same engine and the same distances the reference CAM workbench uses, so the carved
// path matches it).
//
// Inputs are in a scaled integer plane (millimetres × a scale factor, as the V-carve op does the
// scaling) so the construction is exact; vertex coordinates come back as doubles in that same scaled
// plane.
package voronoi

// Point is an input site / a scaled integer coordinate.
type Point struct{ X, Y int64 }

// Segment is an input edge between two points.
type Segment struct{ A, B Point }

// Vertex is a Voronoi vertex in the scaled plane. Valid is false for the open end of an infinite
// edge (the diagram leaves those vertices null).
type Vertex struct {
	X, Y  float64
	Valid bool
}

// Source category values from Boost.Polygon (SOURCE_CATEGORY_*). Categories below the geometry shift
// (8) describe a point site (a single point, or a segment endpoint); 8 and above describe a segment
// site. The V-carve clearance distance is measured to a point site directly and to a segment site by
// orthogonal projection, so the category selects which.
const (
	SourceSinglePoint   = 0 // an inserted point
	SourceSegmentStart  = 1 // the low endpoint of an inserted segment
	SourceSegmentEnd    = 2 // the high endpoint of an inserted segment
	sourceGeometryShift = 8 // categories >= this are segment geometry, not a point
)

// CellSource identifies the input site a Voronoi cell was generated from: its index into the input
// (points first, then segments) and its Boost source category.
type CellSource struct {
	Index    int
	Category int
}

// ContainsPoint reports whether the cell's site is a point (an inserted point or a segment endpoint)
// rather than a whole segment — the test the clearance distance dispatches on.
func (c CellSource) ContainsPoint() bool {
	return c.Category < sourceGeometryShift
}

// Edge is one directed Voronoi edge: its two endpoints (V0/V1, possibly infinite), whether it is a
// primary edge (a real medial edge, not one coincident with an input segment) and linear (a straight
// edge, vs a parabolic arc between a point and a segment site), and the source sites of its cell and
// its twin's cell (both needed to measure the clearance at the edge).
type Edge struct {
	V0, V1    Vertex
	IsPrimary bool
	IsLinear  bool
	Cell      CellSource
	Twin      CellSource
}

// Diagram is the constructed Voronoi diagram as its edge table (edges come in twin pairs, in the
// engine's construction order).
type Diagram struct {
	Edges []Edge
}
