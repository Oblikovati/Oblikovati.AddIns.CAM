// SPDX-License-Identifier: GPL-2.0-only

// Package clipper is the project's thin Go interface over the integer polygon-clipping
// engine that the adaptive-clearing solver is built on. The engine works in a scaled
// integer plane so the boolean (union/difference/intersection) and offset arithmetic is
// exact and order-independent — a floating-point clip of two near-collinear edges is
// ambiguous, the integer version is not.
//
// The heavyweight Vatti boolean/offset engine is the vendored C++ library (BSL-1.0,
// COPYING.clipper) compiled by cgo (see clipper_cgo.go); a non-cgo stub keeps the rest of
// the add-in buildable without a C toolchain. The cheap, standalone predicates in THIS
// file — Area, Orientation, PointInPolygon, ReversePath, CleanPolygon — are pure Go on
// purpose: the clearing solver calls them per toolpath point (CalcCutArea), so paying a
// cgo crossing each time would dominate the run. They are exact integer ports of the same
// library, so they agree bit-for-bit with the engine that produced their inputs.
package clipper

// IntPoint is a point in the engine's scaled integer plane. Real millimetres are mapped in
// by multiplying with a scale factor (the solver uses 100), which is what makes the boolean
// arithmetic exact.
//
// Z is the engine's extra per-vertex tag (the library is built with use_xyz). The adaptive
// solver uses it as a "needs finishing" flag through Execute steps 3–8 (Z=1 a real profile wall
// that wants a finishing pass, Z=0 a stock boundary that does not). It survives a boolean: the
// engine copies a retained vertex's Z and leaves a new intersection vertex at Z=0 (no fill
// callback), exactly matching the upstream union behaviour. An offset drops Z (returns 0), so the
// solver re-stamps it per curve, as upstream does.
type IntPoint struct{ X, Y, Z int64 }

// Path is a single polygon or open polyline as an ordered vertex list; Paths is a set of
// them (an outer contour plus holes, or the disjoint pieces a boolean produced).
type (
	Path  []IntPoint
	Paths []Path
)

// Area returns the signed area of a closed polygon by the shoelace formula. The sign
// encodes winding: positive for a counter-clockwise contour in the engine's coordinate
// convention, negative for clockwise; a degenerate (<3-vertex) path has zero area. The
// intermediate products are taken in float64 to avoid int64 overflow on large scaled
// coordinates, matching ClipperLib's Area().
func Area(poly Path) float64 {
	size := len(poly)
	if size < 3 {
		return 0
	}
	a := 0.0
	for i, j := 0, size-1; i < size; i++ {
		a += (float64(poly[j].X) + float64(poly[i].X)) * (float64(poly[j].Y) - float64(poly[i].Y))
		j = i
	}
	return -a * 0.5
}

// Orientation reports whether a closed polygon winds in the engine's positive
// (counter-clockwise) direction — i.e. whether its signed Area is non-negative. The solver
// uses it to tell an outer boundary from a hole and to normalise offset results.
func Orientation(poly Path) bool {
	return Area(poly) >= 0
}

// PointInPolygon classifies a point against a closed polygon: 0 = outside, +1 = strictly
// inside, -1 = exactly on the boundary. It is the exact integer crossing-number test from
// ClipperLib, robust on the boundary (the -1 case) where a floating-point ray test would be
// ambiguous. The cross-product `d` is taken in float64 to avoid int64 overflow.
func PointInPolygon(pt IntPoint, path Path) int {
	cnt := len(path)
	if cnt < 3 {
		return 0
	}
	result := 0
	ip := path[0]
	for i := 1; i <= cnt; i++ {
		ipNext := path[0]
		if i != cnt {
			ipNext = path[i]
		}
		if ipNext.Y == pt.Y {
			if ipNext.X == pt.X || (ip.Y == pt.Y && ((ipNext.X > pt.X) == (ip.X < pt.X))) {
				return -1
			}
		}
		if (ip.Y < pt.Y) != (ipNext.Y < pt.Y) {
			if ip.X >= pt.X {
				if ipNext.X > pt.X {
					result = 1 - result
				} else {
					d := float64(ip.X-pt.X)*float64(ipNext.Y-pt.Y) - float64(ipNext.X-pt.X)*float64(ip.Y-pt.Y)
					if d == 0 {
						return -1
					}
					if (d > 0) == (ipNext.Y > ip.Y) {
						result = 1 - result
					}
				}
			} else if ipNext.X > pt.X {
				d := float64(ip.X-pt.X)*float64(ipNext.Y-pt.Y) - float64(ipNext.X-pt.X)*float64(ip.Y-pt.Y)
				if d == 0 {
					return -1
				}
				if (d > 0) == (ipNext.Y > ip.Y) {
					result = 1 - result
				}
			}
		}
		ip = ipNext
	}
	return result
}

// ReversePath flips a path's winding in place. The clearing solver reverses contours to put
// an offset result back into the orientation it expects before unioning.
func ReversePath(p Path) {
	for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
		p[i], p[j] = p[j], p[i]
	}
}

// ReversePaths reverses every path in the set in place.
func ReversePaths(p Paths) {
	for i := range p {
		ReversePath(p[i])
	}
}
