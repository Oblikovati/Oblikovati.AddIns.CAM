// SPDX-License-Identifier: GPL-2.0-only

package geom2d

import "math"

// DistanceToBoundary returns the shortest distance from a point to the polygon's boundary — the
// minimum distance to any of its edges. It is the building block for distance-field work such as
// V-carve depth (where the cut depth at a point is set by how far it sits from the nearest wall)
// and medial-axis estimation. A polygon with fewer than two points has no edges; it returns +Inf.
func DistanceToBoundary(pt Point2, poly Polygon) float64 {
	n := len(poly)
	if n < 2 {
		return math.Inf(1)
	}
	best := math.Inf(1)
	for i := 0; i < n; i++ {
		if d := distanceToSegment(pt, poly[i], poly[(i+1)%n]); d < best {
			best = d
		}
	}
	return best
}

// MaxInscribedCircle returns the centre and radius of the largest circle that fits inside the
// polygon — the interior point farthest from the boundary (the "pole of inaccessibility") and that
// farthest distance. The medial axis passes through this centre, and the radius is the region's
// half-width at its widest, which sizes the largest tool that can reach the region and a safe
// plunge/entry point. It coarse-grids the bounding box for interior candidates, then refines around
// the best with a shrinking local search. A polygon with fewer than 3 points returns a zero circle.
func MaxInscribedCircle(poly Polygon) (Point2, float64) {
	if len(poly) < 3 {
		return Point2{}, 0
	}
	minX, minY, maxX, maxY := polygonBounds(poly)
	best, bestD := Point2{}, 0.0
	consider := func(p Point2) {
		if poly.Contains(p) {
			if d := DistanceToBoundary(p, poly); d > bestD {
				best, bestD = p, d
			}
		}
	}
	step := math.Max(maxX-minX, maxY-minY) / 40
	for x := minX; x <= maxX; x += step {
		for y := minY; y <= maxY; y += step {
			consider(Point2{X: x, Y: y})
		}
	}
	for s := step; s > 1e-4; s /= 2 { // refine around the best candidate
		for dx := -1; dx <= 1; dx++ {
			for dy := -1; dy <= 1; dy++ {
				consider(Point2{X: best.X + float64(dx)*s, Y: best.Y + float64(dy)*s})
			}
		}
	}
	return best, bestD
}

// polygonBounds returns the axis-aligned bounding box of a polygon's vertices.
func polygonBounds(poly Polygon) (minX, minY, maxX, maxY float64) {
	minX, minY = poly[0].X, poly[0].Y
	maxX, maxY = poly[0].X, poly[0].Y
	for _, v := range poly[1:] {
		minX, maxX = math.Min(minX, v.X), math.Max(maxX, v.X)
		minY, maxY = math.Min(minY, v.Y), math.Max(maxY, v.Y)
	}
	return minX, minY, maxX, maxY
}

// distanceToSegment returns the distance from p to the segment a→b: the perpendicular distance to
// the segment's line when the foot of the perpendicular lies on the segment, else the distance to
// the nearer endpoint. A degenerate (zero-length) segment is treated as the point a.
func distanceToSegment(p, a, b Point2) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	lenSq := dx*dx + dy*dy
	if lenSq == 0 {
		return dist(p, a)
	}
	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / lenSq
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return dist(p, Point2{X: a.X + t*dx, Y: a.Y + t*dy})
}
