// SPDX-License-Identifier: GPL-2.0-only

//go:build !cgo

package voronoi

import "errors"

// errNoCGO reports that the Voronoi engine needs the cgo build (the vendored Boost.Polygon library is
// C++). The non-cgo build keeps the rest of the add-in compilable without a C toolchain.
var errNoCGO = errors.New("voronoi: the segment-Voronoi engine requires a cgo build")

// Build is the non-cgo stub: it always errors, mirroring the cgo signature.
func Build(points []Point, segments []Segment) (Diagram, error) {
	return Diagram{}, errNoCGO
}
