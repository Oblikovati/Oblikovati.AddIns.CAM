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
