// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import (
	"math"

	"oblikovati.org/cam/bridge/clipper"
)

// samePointTolSqrdScaled is the squared distance (in the scaled integer plane) below which two
// points are treated as coincident when joining/deduplicating link paths.
const samePointTolSqrdScaled = 4.0

// smoothScale is the extra ×1000 integer precision smoothPaths works in (separate from the
// solver's scaleFactor) so the moving average has sub-unit resolution.
const smoothScale = 1000

// getPathNestingLevel counts how many of paths contain path's first point — the even/odd nesting
// used to tell solid from hole and cleared from uncleared. Exact port of getPathNestingLevel.
func getPathNestingLevel(path clipper.Path, paths clipper.Paths) int {
	nesting := 0
	for _, other := range paths {
		if len(path) > 0 && clipper.PointInPolygon(path[0], other) != 0 {
			nesting++
		}
	}
	return nesting
}

// intersectionPoint returns the intersection of segments s1 and s2 if it lies within both, using
// the sign-of-determinant containment test (no division until a hit is confirmed). Exact port of
// the two-segment IntersectionPoint.
func intersectionPoint(s1p1, s1p2, s2p1, s2p2 clipper.IntPoint) (clipper.IntPoint, bool) {
	s1dx := float64(s1p2.X - s1p1.X)
	s1dy := float64(s1p2.Y - s1p1.Y)
	s2dx := float64(s2p2.X - s2p1.X)
	s2dy := float64(s2p2.Y - s2p1.Y)
	d := s1dy*s2dx - s2dy*s1dx
	if math.Abs(d) < numericTolerance {
		return clipper.IntPoint{}, false // parallel
	}
	lpdx := float64(s1p1.X - s2p1.X)
	lpdy := float64(s1p1.Y - s2p1.Y)
	p1d := s2dy*lpdx - s2dx*lpdy
	p2d := s1dy*lpdx - s1dx*lpdy
	if d < 0 && (p1d < d || p1d > 0 || p2d < d || p2d > 0) {
		return clipper.IntPoint{}, false
	}
	if d > 0 && (p1d < 0 || p1d > d || p2d < 0 || p2d > d) {
		return clipper.IntPoint{}, false
	}
	t := p1d / d
	return clipper.IntPoint{X: s1p1.X + int64(s1dx*t), Y: s1p1.Y + int64(s1dy*t)}, true
}

// intersectionPointPaths returns the first intersection of segment p1→p2 with any (closed) path,
// using a growing bounding box per path to skip non-colliding spans. Exact port of the
// paths-overload IntersectionPoint.
func intersectionPointPaths(paths clipper.Paths, p1, p2 clipper.IntPoint) (clipper.IntPoint, bool) {
	segBB := newBoundBoxPoint(p1)
	segBB.addPoint(p2)
	for _, path := range paths {
		size := len(path)
		if size < 2 {
			continue
		}
		pathBB := newBoundBoxPoint(path[0])
		for j := 0; j < size; j++ {
			pp2 := path[j]
			pathBB.addPoint(pp2)
			if !pathBB.collidesWith(segBB) {
				continue
			}
			prev := size - 1
			if j > 0 {
				prev = j - 1
			}
			pp1 := path[prev]
			ldy := float64(p2.Y - p1.Y)
			ldx := float64(p2.X - p1.X)
			pdx := float64(pp2.X - pp1.X)
			pdy := float64(pp2.Y - pp1.Y)
			d := ldy*pdx - pdy*ldx
			if math.Abs(d) < numericTolerance {
				continue // parallel
			}
			lpdx := float64(p1.X - pp1.X)
			lpdy := float64(p1.Y - pp1.Y)
			p1d := pdy*lpdx - pdx*lpdy
			p2d := ldy*lpdx - ldx*lpdy
			if d < 0 && (p1d < d || p1d > 0 || p2d < d || p2d > 0) {
				continue
			}
			if d > 0 && (p1d < 0 || p1d > d || p2d < 0 || p2d > d) {
				continue
			}
			t := p1d / d
			return clipper.IntPoint{X: p1.X + int64(ldx*t), Y: p1.Y + int64(ldy*t)}, true
		}
	}
	return clipper.IntPoint{}, false
}

// prependReversed returns reverse(n) followed by joined — the effect of inserting each point of n
// at the front of joined one at a time (as ConnectPaths does for front-joining).
func prependReversed(joined, n clipper.Path) clipper.Path {
	out := make(clipper.Path, 0, len(joined)+len(n))
	for i := len(n) - 1; i >= 0; i-- {
		out = append(out, n[i])
	}
	return append(out, joined...)
}

// connectPaths greedily welds paths whose endpoints coincide into longer chains, reversing a path
// when needed so the touching ends meet. Exact port of ConnectPaths (the input is deep-copied so a
// reversal never mutates the caller's data).
func connectPaths(input clipper.Paths) clipper.Paths {
	work := make(clipper.Paths, len(input))
	for i, p := range input {
		work[i] = append(clipper.Path(nil), p...)
	}
	var output clipper.Paths
	newPath := true
	var joined clipper.Path
	for len(work) > 0 {
		if newPath {
			if len(joined) > 0 {
				output = append(output, joined)
			}
			joined = append(clipper.Path(nil), work[0]...)
			work = work[1:]
			newPath = false
		}
		anyMatch := false
		for i := range work {
			n := work[i]
			switch {
			case distanceSqrd(n[0], joined[len(joined)-1]) < samePointTolSqrdScaled:
				joined = append(joined, n...)
			case distanceSqrd(n[len(n)-1], joined[len(joined)-1]) < samePointTolSqrdScaled:
				clipper.ReversePath(n)
				joined = append(joined, n...)
			case distanceSqrd(n[0], joined[0]) < samePointTolSqrdScaled:
				joined = prependReversed(joined, n)
			case distanceSqrd(n[len(n)-1], joined[0]) < samePointTolSqrdScaled:
				clipper.ReversePath(n)
				joined = prependReversed(joined, n)
			default:
				continue
			}
			work = append(work[:i], work[i+1:]...)
			anyMatch = true
			break
		}
		if !anyMatch {
			newPath = true
		}
	}
	if len(joined) > 0 {
		output = append(output, joined)
	}
	return output
}

// deduplicatePaths drops a path when every one of its points already lies on an earlier kept path.
// Exact port of DeduplicatePaths.
func deduplicatePaths(inputs clipper.Paths) clipper.Paths {
	var outputs clipper.Paths
	for _, newPth := range inputs {
		duplicate := false
		for _, oldPth := range outputs {
			allExist := true
			for _, pt1 := range newPth {
				exists := false
				for _, pt2 := range oldPth {
					if distanceSqrd(pt1, pt2) < samePointTolSqrdScaled {
						exists = true
						break
					}
				}
				if !exists {
					allExist = false
					break
				}
			}
			if allExist {
				duplicate = true
				break
			}
		}
		if !duplicate && len(newPth) > 0 {
			outputs = append(outputs, newPth)
		}
	}
	return outputs
}

// popPathWithClosestPoint finds the path with the point closest to p1, rotates it to start at that
// point (optionally walking extraDistanceAround further along it first), and returns it with that
// path removed from the collection. Exact port of PopPathWithClosestPoint.
func popPathWithClosestPoint(paths clipper.Paths, p1 clipper.IntPoint, extraDistanceAround float64) (clipper.Path, clipper.Paths, bool) {
	if len(paths) == 0 {
		return nil, paths, false
	}
	minDist := math.MaxFloat64
	closestPathIndex := 0
	closestPointIndex := 0
	for pi, path := range paths {
		for i, pt := range path {
			if dist := distanceSqrd(p1, pt); dist < minDist {
				minDist = dist
				closestPathIndex = pi
				closestPointIndex = i
			}
		}
	}
	closestPath := paths[closestPathIndex]
	for extraDistanceAround > 0 {
		nexti := (closestPointIndex + 1) % len(closestPath)
		extraDistanceAround -= math.Sqrt(distanceSqrd(closestPath[closestPointIndex], closestPath[nexti]))
		closestPointIndex = nexti
	}
	result := make(clipper.Path, 0, len(closestPath))
	for i := 0; i < len(closestPath); i++ {
		result = append(result, closestPath[(closestPointIndex+i)%len(closestPath)])
	}
	remaining := append(clipper.Paths(nil), paths[:closestPathIndex]...)
	remaining = append(remaining, paths[closestPathIndex+1:]...)
	return result, remaining, true
}

// cleanPath simplifies a single path (collinear/near-coincident removal) while preserving its
// original start and end points and orientation. Exact port of CleanPath.
func cleanPath(inp clipper.Path, tolerance float64) clipper.Path {
	if len(inp) < 3 {
		return append(clipper.Path(nil), inp...)
	}
	tmp := clipper.CleanPolygon(inp, tolerance)
	size := len(tmp)
	// CleanPolygon collapses an all-collinear path to nothing; keep the endpoints.
	if size <= 2 {
		return clipper.Path{inp[0], inp[len(inp)-1]}
	}
	near := distancePointToPathsSqrd(clipper.Paths{tmp}, inp[0])
	clp := near.point
	seg := near.segmentIndex
	prev := size - 1
	if seg > 0 {
		prev = seg - 1
	}
	var out clipper.Path
	// if the closest point isn't already a vertex, add it as the new first point
	if distanceSqrd(clp, tmp[seg]) > 0 && distanceSqrd(clp, tmp[prev]) > 0 {
		out = append(out, clp)
	}
	for i := 0; i < size; i++ {
		out = append(out, tmp[(seg+i)%size])
	}
	if distanceSqrd(out[0], inp[0]) > samePointTolSqrdScaled {
		out = prependReversed(out, clipper.Path{inp[0]})
	}
	if distanceSqrd(out[len(out)-1], inp[len(inp)-1]) > samePointTolSqrdScaled {
		out = append(out, inp[len(inp)-1])
	}
	return out
}

// indexedPoint is one smoothed point and the path it belongs to.
type indexedPoint struct {
	pathIndex int
	pt        clipper.IntPoint
}

// smoothPaths resamples the paths at stepSize and runs a windowed moving average, keeping the very
// ends sharp (only the interior is averaged). Operates in a ×1000 plane for sub-unit precision and
// returns the smoothed paths. Exact port of SmoothPaths.
func smoothPaths(paths clipper.Paths, stepSize float64, pointCount, iterations int) clipper.Paths {
	stepScaled := stepSize * smoothScale
	output := make(clipper.Paths, len(paths))

	scaleUpPaths(paths, smoothScale)
	points := resamplePoints(paths, stepScaled, pointCount, iterations)
	if len(points) == 0 {
		scaleDownPaths(paths, smoothScale)
		return paths
	}
	averagePoints(points, pointCount, iterations)
	for _, pr := range points {
		output[pr.pathIndex] = append(output[pr.pathIndex], pr.pt)
	}
	for i := range paths {
		paths[i] = cleanPath(output[i], 1.4*smoothScale)
	}
	scaleDownPaths(paths, smoothScale)
	return paths
}

// resamplePoints densifies the (scaled-up) paths to roughly stepScaled spacing, but skips the
// densification in the middle of long spans (only the first/last pointCount*iterations*2 samples on
// each end are emitted) since only the ends need extra points to smooth. Exact port of the
// resampling loop in SmoothPaths.
func resamplePoints(paths clipper.Paths, stepScaled float64, pointCount, iterations int) []indexedPoint {
	var points []indexedPoint
	for i := range paths {
		for _, pt := range paths[i] {
			if len(points) == 0 {
				points = append(points, indexedPoint{i, pt})
				continue
			}
			back := points[len(points)-1]
			l := math.Sqrt(distanceSqrd(back.pt, pt))
			if l < 0.5*stepScaled {
				if len(points) > 1 {
					points = points[:len(points)-1]
				}
				points = append(points, indexedPoint{i, pt})
				continue
			}
			steps := int(math.Max(l/stepScaled, 1))
			left := pointCount * iterations * 2
			right := steps - pointCount*iterations*2
			for idx := 0; idx <= steps; idx++ {
				if idx > left && idx < right {
					idx = right
					continue
				}
				p := float64(idx) / float64(steps)
				ptx := clipper.IntPoint{
					X: back.pt.X + int64(float64(pt.X-back.pt.X)*p),
					Y: back.pt.Y + int64(float64(pt.Y-back.pt.Y)*p),
				}
				if idx == 0 && distanceSqrd(back.pt, ptx) < smoothScale && len(points) > 1 {
					points = points[:len(points)-1]
				}
				if p < 0.5 {
					points = append(points, indexedPoint{back.pathIndex, ptx})
				} else {
					points = append(points, indexedPoint{i, ptx})
				}
			}
		}
	}
	return points
}

// averagePoints runs the in-place windowed moving average over the interior points (the ends are
// left untouched), narrowing the window near the ends. Exact port of the averaging loop.
func averagePoints(points []indexedPoint, pointCount, iterations int) {
	size := len(points)
	for iter := 0; iter < iterations; iter++ {
		for i := 1; i < size-1; i++ {
			avgX, avgY := points[i].pt.X, points[i].pt.Y
			cnt := int64(1)
			ptsToAverage := pointCount
			if i <= ptsToAverage {
				ptsToAverage = max(i-1, 0)
			} else if i+ptsToAverage >= size-1 {
				ptsToAverage = size - 1 - i
			}
			for j := i - ptsToAverage; j <= i+ptsToAverage; j++ {
				if j == i {
					continue
				}
				index := min(max(j, 0), size-1)
				avgX += points[index].pt.X
				avgY += points[index].pt.Y
				cnt++
			}
			points[i].pt = clipper.IntPoint{X: avgX / cnt, Y: avgY / cnt}
		}
	}
}
