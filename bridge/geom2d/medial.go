// SPDX-License-Identifier: GPL-2.0-only

package geom2d

import "math"

// MedialAxisPoints returns a sampled approximation of the polygon's medial axis (its skeleton): the
// interior points that are roughly equidistant from two distinct parts of the boundary. It samples
// the interior on a grid of the given spacing and keeps the cells where the second-nearest boundary
// feature is about as close as the nearest one — the defining property of a medial point — which
// captures ridges in any orientation (the diagonal spokes of a square as well as the centreline of a
// slot), unlike a simple axis-aligned local-maximum test.
//
// The medial axis is the backbone for corner-relief and constant-engagement work: each medial point
// carries the largest tool that fits there (its distance to the boundary). A spacing <= 0, a
// sub-triangle, or a region thinner than the spacing returns no points.
func MedialAxisPoints(poly Polygon, spacing float64) []Point2 {
	if len(poly) < 3 || spacing <= 0 {
		return nil
	}
	minX, minY, maxX, maxY := polygonBounds(poly)
	var pts []Point2
	for x := minX; x <= maxX; x += spacing {
		for y := minY; y <= maxY; y += spacing {
			p := Point2{X: x, Y: y}
			if poly.Contains(p) && isMedial(p, poly, spacing) {
				pts = append(pts, p)
			}
		}
	}
	return pts
}

// medialTolerance is how much farther the second feature may be than the nearest one for the point
// to still count as equidistant (15%) — loose enough to survive grid sampling, tight enough not to
// smear the ridge into a thick band.
const medialTolerance = 0.15

// isMedial reports whether p is roughly equidistant from two distinct boundary features: its nearest
// boundary point, and another boundary point at least the clearance away from the first (so they are
// genuinely different walls/corners, not two samples of the same edge) that is nearly as close. The
// d1 > spacing guard drops grid noise hugging a wall, where every point trivially has one near edge.
func isMedial(p Point2, poly Polygon, spacing float64) bool {
	n1, d1 := closestBoundaryPoint(p, poly)
	if d1 <= spacing {
		return false
	}
	n := len(poly)
	for i := 0; i < n; i++ {
		q := closestPointOnSegment(p, poly[i], poly[(i+1)%n])
		if dist(q, n1) <= d1 { // same feature as the nearest — skip
			continue
		}
		if dist(p, q) <= d1*(1+medialTolerance) { // a second, distinct wall just as close → medial
			return true
		}
	}
	return false
}

// closestBoundaryPoint returns the point on the polygon boundary nearest p and its distance.
func closestBoundaryPoint(p Point2, poly Polygon) (Point2, float64) {
	n := len(poly)
	best := poly[0]
	bestD := math.Inf(1)
	for i := 0; i < n; i++ {
		q := closestPointOnSegment(p, poly[i], poly[(i+1)%n])
		if d := dist(p, q); d < bestD {
			best, bestD = q, d
		}
	}
	return best, bestD
}

// closestPointOnSegment returns the point on segment a→b nearest p (the foot of the perpendicular,
// clamped to the segment ends). A degenerate segment returns a.
func closestPointOnSegment(p, a, b Point2) Point2 {
	dx, dy := b.X-a.X, b.Y-a.Y
	lenSq := dx*dx + dy*dy
	if lenSq == 0 {
		return a
	}
	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / lenSq
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return Point2{X: a.X + t*dx, Y: a.Y + t*dy}
}
