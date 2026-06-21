// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

package voronoi

/*
// cgo links libstdc++/libc++ automatically because this package contains C++ sources. -DNDEBUG
// compiles out the library's asserts so a degenerate input degrades gracefully. The vendored Boost
// headers are found via the vendor include dir.
#cgo CXXFLAGS: -I${SRCDIR}/vendor -std=c++14 -O2 -DNDEBUG
#include <stdlib.h>
#include "wrapper.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// Build constructs the segment Voronoi diagram of the given points and segments (scaled integer
// coordinates that must fit in 32 bits) and returns its edge table. Backed by the vendored
// Boost.Polygon Voronoi engine.
func Build(points []Point, segments []Segment) (Diagram, error) {
	pc := flattenPoints(points)
	sc := flattenSegments(segments)
	var outCoords *C.double
	var outMeta *C.longlong
	n := C.obk_voronoi_build(
		ptrLL(pc), C.int(len(points)),
		ptrLL(sc), C.int(len(segments)),
		&outCoords, &outMeta)
	if n < 0 {
		return Diagram{}, fmt.Errorf("voronoi: build of %d points / %d segments failed", len(points), len(segments))
	}
	return collectEdges(outCoords, outMeta, int(n)), nil
}

// flattenPoints packs points as 2 int64 per point (x,y); an empty input yields a one-element slice so
// &slice[0] is addressable for the cgo call.
func flattenPoints(points []Point) []C.longlong {
	out := make([]C.longlong, max1(len(points)*2))
	for i, p := range points {
		out[2*i] = C.longlong(p.X)
		out[2*i+1] = C.longlong(p.Y)
	}
	return out
}

// flattenSegments packs segments as 4 int64 per segment (x0,y0,x1,y1).
func flattenSegments(segments []Segment) []C.longlong {
	out := make([]C.longlong, max1(len(segments)*4))
	for i, s := range segments {
		out[4*i] = C.longlong(s.A.X)
		out[4*i+1] = C.longlong(s.A.Y)
		out[4*i+2] = C.longlong(s.B.X)
		out[4*i+3] = C.longlong(s.B.Y)
	}
	return out
}

// collectEdges rebuilds the edge table from the malloc'd coords/meta arrays and frees them.
func collectEdges(outCoords *C.double, outMeta *C.longlong, nedges int) Diagram {
	defer C.obk_voronoi_free_d(outCoords)
	defer C.obk_voronoi_free_ll(outMeta)
	if nedges == 0 {
		return Diagram{}
	}
	coords := unsafe.Slice((*float64)(unsafe.Pointer(outCoords)), nedges*4)
	meta := unsafe.Slice((*int64)(unsafe.Pointer(outMeta)), nedges*8)
	edges := make([]Edge, nedges)
	for i := 0; i < nedges; i++ {
		c, m := coords[i*4:], meta[i*8:]
		edges[i] = Edge{
			V0:        Vertex{X: c[0], Y: c[1], Valid: m[0] != 0},
			V1:        Vertex{X: c[2], Y: c[3], Valid: m[1] != 0},
			IsPrimary: m[2] != 0,
			IsLinear:  m[3] != 0,
			Cell:      CellSource{Index: int(m[4]), Category: int(m[5])},
			Twin:      CellSource{Index: int(m[6]), Category: int(m[7])},
		}
	}
	return Diagram{Edges: edges}
}

func ptrLL(s []C.longlong) *C.longlong { return &s[0] }

// max1 returns n, or 1 when n is 0, so an allocation is never zero-length.
func max1(n int) int {
	if n == 0 {
		return 1
	}
	return n
}
