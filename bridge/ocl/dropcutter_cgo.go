// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

package ocl

/*
// cgo links the C++ standard library automatically because this package contains C++ (.cpp)
// sources, so LDFLAGS need not name it (which would be libstdc++ on Linux/MinGW but libc++ on
// macOS) — keeping the build portable across the CI matrix.
#cgo CXXFLAGS: -std=c++14 -O2 -include cassert -I${SRCDIR}/shim
#include <stdlib.h>
#include "wrapper.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// DropCutter drops a ball-nose cutter (diameter, length in mm) along the given XY scan lines
// over the triangle mesh, following the surface down to minZ and sampling each pass at the
// given step. It returns the cutter-location points grouped per scan line. Backed by
// OpenCAMLib's PathDropCutter.
func DropCutter(tris []Triangle, diameter, length, minZ, sampling float64, lines []ScanLine) ([][]Point3, error) {
	if len(tris) == 0 {
		return nil, fmt.Errorf("ocl: empty mesh (no triangles)")
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("ocl: no scan lines to drop")
	}
	if diameter <= 0 || sampling <= 0 {
		return nil, fmt.Errorf("ocl: diameter %g and sampling %g must be positive", diameter, sampling)
	}
	flatT := flattenTriangles(tris)
	flatS := flattenLines(lines)

	var outXYZ *C.double
	var outCounts *C.int
	n := C.obk_ocl_drop_lines(
		(*C.double)(&flatT[0]), C.int(len(tris)),
		C.double(diameter), C.double(length), C.double(minZ), C.double(sampling),
		(*C.double)(&flatS[0]), C.int(len(lines)),
		&outXYZ, &outCounts)
	if n < 0 {
		return nil, fmt.Errorf("ocl: drop-cutter failed for %d triangles / %d scan lines", len(tris), len(lines))
	}
	defer C.obk_ocl_free_d(outXYZ)
	defer C.obk_ocl_free_i(outCounts)
	return groupRows(outXYZ, outCounts, int(n), len(lines)), nil
}

// flattenTriangles packs triangles into the 9-doubles-per-triangle layout the C side expects.
func flattenTriangles(tris []Triangle) []C.double {
	flat := make([]C.double, len(tris)*9)
	for i, t := range tris {
		base := i * 9
		for k := 0; k < 3; k++ {
			flat[base+k] = C.double(t.A[k])
			flat[base+3+k] = C.double(t.B[k])
			flat[base+6+k] = C.double(t.C[k])
		}
	}
	return flat
}

// flattenLines packs scan lines into the 4-doubles-per-line layout the C side expects.
func flattenLines(lines []ScanLine) []C.double {
	flat := make([]C.double, len(lines)*4)
	for i, l := range lines {
		flat[i*4], flat[i*4+1], flat[i*4+2], flat[i*4+3] = C.double(l.X0), C.double(l.Y0), C.double(l.X1), C.double(l.Y1)
	}
	return flat
}

// groupRows slices the flat xyz output into per-scan-line point rows using the counts array.
func groupRows(outXYZ *C.double, outCounts *C.int, total, nlines int) [][]Point3 {
	xyz := unsafe.Slice((*float64)(unsafe.Pointer(outXYZ)), total*3)
	counts := unsafe.Slice((*int32)(unsafe.Pointer(outCounts)), nlines)
	rows := make([][]Point3, nlines)
	idx := 0
	for i := 0; i < nlines; i++ {
		c := int(counts[i])
		row := make([]Point3, c)
		for j := 0; j < c; j++ {
			row[j] = Point3{X: xyz[idx*3], Y: xyz[idx*3+1], Z: xyz[idx*3+2]}
			idx++
		}
		rows[i] = row
	}
	return rows
}
