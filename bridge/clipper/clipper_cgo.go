// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

package clipper

/*
// cgo links the C++ standard library automatically because this package contains C++ (.cpp)
// sources, so LDFLAGS need not name it (libstdc++ on Linux/MinGW, libc++ on macOS) — keeping
// the build portable across the CI matrix. -DNDEBUG compiles out the library's asserts so a
// degenerate input degrades gracefully instead of abort()ing the host process.
#cgo CXXFLAGS: -std=c++14 -O2 -DNDEBUG
#include <stdlib.h>
#include "wrapper.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// Boolean clips subjects against clips with the given operation and fill rule. subjClosed treats
// the subjects as closed polygons (the usual case); returnOpen asks for the open paths the
// boolean produced (used to clip an open toolpath against a region) instead of the closed
// solution. Backed by the vendored Clipper engine.
func Boolean(clip ClipType, fill FillType, subjects Paths, subjClosed bool, clips Paths, returnOpen bool) (Paths, error) {
	subjCoords, subjCounts := flatten(subjects)
	clipCoords, clipCounts := flatten(clips)
	var outCounts *C.int
	var outCoords *C.longlong
	n := C.obk_clipper_boolean(
		C.int(clip), C.int(fill),
		ptr(subjCoords), ptrI(subjCounts), C.int(len(subjects)), boolToInt(subjClosed),
		ptr(clipCoords), ptrI(clipCounts), C.int(len(clips)),
		boolToInt(returnOpen),
		&outCounts, &outCoords)
	if n < 0 {
		return nil, fmt.Errorf("clipper: boolean op %d on %d subject / %d clip paths failed", clip, len(subjects), len(clips))
	}
	return collect(outCounts, outCoords, int(n)), nil
}

// Offset insets/outsets the paths by delta integer units with the given join/end style.
// miterLimit and arcTolerance pass through to ClipperOffset (<=0 uses the library defaults).
func Offset(paths Paths, join JoinType, end EndType, delta, miterLimit, arcTolerance float64) (Paths, error) {
	coords, counts := flatten(paths)
	var outCounts *C.int
	var outCoords *C.longlong
	n := C.obk_clipper_offset(
		ptr(coords), ptrI(counts), C.int(len(paths)),
		C.int(join), C.int(end), C.double(delta),
		C.double(miterLimit), C.double(arcTolerance),
		&outCounts, &outCoords)
	if n < 0 {
		return nil, fmt.Errorf("clipper: offset by %g of %d paths failed", delta, len(paths))
	}
	return collect(outCounts, outCoords, int(n)), nil
}

// Simplify resolves self-intersections in the path set (Clipper's SimplifyPolygons).
func Simplify(paths Paths, fill FillType) (Paths, error) {
	coords, counts := flatten(paths)
	var outCounts *C.int
	var outCoords *C.longlong
	n := C.obk_clipper_simplify(ptr(coords), ptrI(counts), C.int(len(paths)), C.int(fill), &outCounts, &outCoords)
	if n < 0 {
		return nil, fmt.Errorf("clipper: simplify of %d paths failed", len(paths))
	}
	return collect(outCounts, outCoords, int(n)), nil
}

// PathIntersectArea intersects one open subject path with the closed-area set obj, returning the
// open sub-paths of subject that lie inside the area, each keeping subject's traversal direction.
// It is the open-toolpath-clipped-by-region primitive the adaptive engagement search uses; the
// direction preservation and seam rejoin happen inside the engine (via its order/Z field).
func PathIntersectArea(subject Path, obj Paths) (Paths, error) {
	subjCoords, _ := flatten(Paths{subject})
	objCoords, objCounts := flatten(obj)
	var outCounts *C.int
	var outCoords *C.longlong
	n := C.obk_clipper_path_intersect_area(
		ptr(subjCoords), C.int(len(subject)),
		ptr(objCoords), ptrI(objCounts), C.int(len(obj)),
		&outCounts, &outCoords)
	if n < 0 {
		return nil, fmt.Errorf("clipper: path-intersect-area of a %d-point path with %d area paths failed", len(subject), len(obj))
	}
	return collect(outCounts, outCoords, int(n)), nil
}

// flatten packs a Paths set into the (coords, counts) layout the C side expects: coords holds 2
// int64 per point (x,y interleaved), counts holds each path's point count. Empty input yields
// one-element slices so &slice[0] is always addressable for the cgo call.
func flatten(paths Paths) ([]C.longlong, []C.int) {
	total := 0
	for _, p := range paths {
		total += len(p)
	}
	coords := make([]C.longlong, max1(total*2))
	counts := make([]C.int, max1(len(paths)))
	k := 0
	for i, p := range paths {
		counts[i] = C.int(len(p))
		for _, pt := range p {
			coords[k] = C.longlong(pt.X)
			coords[k+1] = C.longlong(pt.Y)
			k += 2
		}
	}
	return coords, counts
}

// collect rebuilds npaths result paths from the malloc'd C arrays and frees them.
func collect(outCounts *C.int, outCoords *C.longlong, npaths int) Paths {
	defer C.obk_clipper_free_i(outCounts)
	defer C.obk_clipper_free_ll(outCoords)
	if npaths == 0 {
		return Paths{}
	}
	counts := unsafe.Slice((*int32)(unsafe.Pointer(outCounts)), npaths)
	total := 0
	for _, c := range counts {
		total += int(c)
	}
	coords := unsafe.Slice((*int64)(unsafe.Pointer(outCoords)), total*2)
	result := make(Paths, npaths)
	idx := 0
	for i := 0; i < npaths; i++ {
		c := int(counts[i])
		path := make(Path, c)
		for j := 0; j < c; j++ {
			path[j] = IntPoint{X: coords[idx*2], Y: coords[idx*2+1]}
			idx++
		}
		result[i] = path
	}
	return result
}

func ptr(s []C.longlong) *C.longlong { return &s[0] }
func ptrI(s []C.int) *C.int          { return &s[0] }

func boolToInt(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

// max1 returns n, or 1 when n is 0, so an allocation is never zero-length.
func max1(n int) int {
	if n == 0 {
		return 1
	}
	return n
}
