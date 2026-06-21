// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import (
	"math"

	"oblikovati.org/cam/bridge/clipper"
)

// closestOnPaths is the nearest point on a set of closed paths to a query point: which path and
// segment it lies on (the segment ending at vertex segmentIndex, wrapping), its parameter along
// that segment, and the squared distance.
type closestOnPaths struct {
	distSqrd     float64
	point        clipper.IntPoint
	pathIndex    int
	segmentIndex int
	parameter    float64
}

// distancePointToPathsSqrd finds the closest point on any segment of paths to pt. The solver uses
// it every step to measure the tool's distance to the boundary (to slow the step near walls) and
// the local boundary direction. Exact port of DistancePointToPathsSqrd (each path is closed, so
// segment j runs from vertex j-1, wrapping, to vertex j).
func distancePointToPathsSqrd(paths clipper.Paths, pt clipper.IntPoint) closestOnPaths {
	best := closestOnPaths{distSqrd: math.MaxFloat64}
	for i, path := range paths {
		size := len(path)
		for j := 0; j < size; j++ {
			prev := size - 1
			if j > 0 {
				prev = j - 1
			}
			distSq, clp, param := distancePointToLineSegSquared(path[prev], path[j], pt, true)
			if distSq < best.distSqrd {
				best = closestOnPaths{distSqrd: distSq, point: clp, pathIndex: i, segmentIndex: j, parameter: param}
			}
		}
	}
	return best
}
