// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import (
	"math"

	"oblikovati.org/cam/bridge/clipper"
)

// The scaled-plane vector/geometry primitives the solver runs on. They operate on
// clipper.IntPoint (the exact integer plane) and DoublePoint (unit directions / real space),
// and are exact ports of the Adaptive2d inline utilities so the toolpath geometry matches.

// distanceSqrd is the squared Euclidean distance between two scaled points (float64 to avoid
// int64 overflow on large coordinates).
func distanceSqrd(p1, p2 clipper.IntPoint) float64 {
	dx := float64(p1.X - p2.X)
	dy := float64(p1.Y - p2.Y)
	return dx*dx + dy*dy
}

// distanceBetween is the Euclidean distance between two scaled points.
func distanceBetween(p1, p2 clipper.IntPoint) float64 {
	return math.Sqrt(distanceSqrd(p1, p2))
}

// setSegmentLength moves pt2 along the p1→pt2 direction so the segment has new_length, in place.
// Returns false (leaving pt2 untouched) for a zero-length segment.
func setSegmentLength(p1 clipper.IntPoint, pt2 *clipper.IntPoint, newLength float64) bool {
	dx := float64(pt2.X - p1.X)
	dy := float64(pt2.Y - p1.Y)
	l := math.Sqrt(dx*dx + dy*dy)
	if l <= 0 {
		return false
	}
	pt2.X = p1.X + int64(newLength*dx/l)
	pt2.Y = p1.Y + int64(newLength*dy/l)
	return true
}

// rotateVec rotates a direction by rad radians (CCW).
func rotateVec(in DoublePoint, rad float64) DoublePoint {
	c, s := math.Cos(rad), math.Sin(rad)
	return DoublePoint{X: c*in.X - s*in.Y, Y: s*in.X + c*in.Y}
}

// pathLength is the polyline length of an open path.
func pathLength(path clipper.Path) float64 {
	length := 0.0
	for i := 1; i < len(path); i++ {
		length += distanceBetween(path[i-1], path[i])
	}
	return length
}

// pointSideOfLine returns the signed area term of (p1,p2,pt): positive on one side of the line
// p1→p2, negative on the other, zero on it.
func pointSideOfLine(p1, p2, pt clipper.IntPoint) float64 {
	return float64((pt.X-p1.X)*(p2.Y-p1.Y) - (pt.Y-p2.Y)*(p2.X-p1.X))
}

// angle3Points is the turning angle at p2 of the path p1→p2→p3, in [0, π].
func angle3Points(p1, p2, p3 DoublePoint) float64 {
	t1 := math.Atan2(p2.Y-p1.Y, p2.X-p1.X)
	t2 := math.Atan2(p3.Y-p2.Y, p3.X-p2.X)
	a := math.Abs(t2 - t1)
	return math.Min(a, 2*math.Pi-a)
}

// directionV is the unit direction from pt1 to pt2, or the zero vector for coincident points.
func directionV(pt1, pt2 clipper.IntPoint) DoublePoint {
	dx := float64(pt2.X - pt1.X)
	dy := float64(pt2.Y - pt1.Y)
	l := math.Sqrt(dx*dx + dy*dy)
	if l < numericTolerance {
		return DoublePoint{}
	}
	return DoublePoint{X: dx / l, Y: dy / l}
}

// normalizeV scales a vector to unit length in place (a near-zero vector is left unchanged).
func normalizeV(pt *DoublePoint) {
	l := math.Sqrt(pt.X*pt.X + pt.Y*pt.Y)
	if l > numericTolerance {
		pt.X /= l
		pt.Y /= l
	}
}

// getPathDirectionV is the unit direction of the edge ENTERING vertex pointIndex (wrapping to
// the last vertex for index 0), or zero for a degenerate path.
func getPathDirectionV(path clipper.Path, pointIndex int) DoublePoint {
	if len(path) < 2 {
		return DoublePoint{}
	}
	prev := pointIndex - 1
	if pointIndex == 0 {
		prev = len(path) - 1
	}
	return directionV(path[prev], path[pointIndex])
}

// pointsCoincident reports whether two scaled points are within one unit on each axis — the
// solver's "isClose" same-point test.
func pointsCoincident(a, b clipper.IntPoint) bool {
	return absI64(a.X-b.X) <= 1 && absI64(a.Y-b.Y) <= 1
}

// translatePath returns a copy of input shifted by delta.
func translatePath(input clipper.Path, delta clipper.IntPoint) clipper.Path {
	out := make(clipper.Path, len(input))
	for i, p := range input {
		out[i] = clipper.IntPoint{X: p.X + delta.X, Y: p.Y + delta.Y}
	}
	return out
}

// averageDirection returns the normalised sum of unit vectors (their mean heading). An empty
// input or a set that sums to zero yields the zero vector.
func averageDirection(unityVectors []DoublePoint) DoublePoint {
	var out DoublePoint
	for _, v := range unityVectors {
		out.X += v.X
		out.Y += v.Y
	}
	magnitude := math.Sqrt(out.X*out.X + out.Y*out.Y)
	if magnitude < numericTolerance {
		return DoublePoint{}
	}
	out.X /= magnitude
	out.Y /= magnitude
	return out
}

// averageDV is the arithmetic mean of a slice (0 for empty).
func averageDV(vec []float64) float64 {
	if len(vec) == 0 {
		return 0
	}
	s := 0.0
	for _, v := range vec {
		s += v
	}
	return s / float64(len(vec))
}

// distancePointToLineSegSquared returns the squared distance from pt to the segment p1→p2, the
// closest point on it, and the normalised parameter (0 at p1, 1 at p2) of that closest point.
// With clamp, the closest point is restricted to the segment; without it, to the infinite line.
func distancePointToLineSegSquared(p1, p2, pt clipper.IntPoint, clamp bool) (distSqrd float64, closest clipper.IntPoint, param float64) {
	d21x := float64(p2.X - p1.X)
	d21y := float64(p2.Y - p1.Y)
	dp1x := float64(pt.X - p1.X)
	dp1y := float64(pt.Y - p1.Y)
	lsegLenSqr := d21x*d21x + d21y*d21y
	if lsegLenSqr == 0 { // zero-length segment → point-to-point distance
		return dp1x*dp1x + dp1y*dp1y, p1, 0
	}
	parameter := dp1x*d21x + dp1y*d21y
	if clamp {
		if parameter < 0 {
			parameter = 0
		} else if parameter > lsegLenSqr {
			parameter = lsegLenSqr
		}
	}
	param = parameter / lsegLenSqr
	closest = clipper.IntPoint{X: p1.X + int64(param*d21x), Y: p1.Y + int64(param*d21y)}
	dx := float64(pt.X - closest.X)
	dy := float64(pt.Y - closest.Y)
	return dx*dx + dy*dy, closest, param
}

// scaleUpPaths multiplies every coordinate by scaleFactor in place (real → scaled plane).
func scaleUpPaths(paths clipper.Paths, scaleFactor int64) {
	for i := range paths {
		for j := range paths[i] {
			paths[i][j].X *= scaleFactor
			paths[i][j].Y *= scaleFactor
		}
	}
}

// scaleDownPaths integer-divides every coordinate by scaleFactor in place (scaled → real).
func scaleDownPaths(paths clipper.Paths, scaleFactor int64) {
	for i := range paths {
		for j := range paths[i] {
			paths[i][j].X /= scaleFactor
			paths[i][j].Y /= scaleFactor
		}
	}
}

// absI64 is the int64 absolute value.
func absI64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
