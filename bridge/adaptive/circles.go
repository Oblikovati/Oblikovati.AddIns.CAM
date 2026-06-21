// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import (
	"math"

	"oblikovati.org/cam/bridge/clipper"
)

// The circle/line intersection primitives the engagement integration uses to find where the two
// tool circles (previous and new position) cross the cleared-region edges and each other. Exact
// ports; all coordinates are in the scaled integer plane, returned as DoublePoint.

// circle2CircleIntersect returns the two intersection points of two equal-radius circles centred
// at c1 and c2. It reports ok=false for coincident centres OR when the centres are radius-or-more
// apart — the solver only needs the case where the two tool positions are within a stepover (much
// less than the radius), so this stricter guard matches the original.
func circle2CircleIntersect(c1, c2 clipper.IntPoint, radius float64) (first, second DoublePoint, ok bool) {
	dx := float64(c2.X - c1.X)
	dy := float64(c2.Y - c1.Y)
	d := math.Sqrt(dx*dx + dy*dy)
	if d < numericTolerance || d >= radius {
		return DoublePoint{}, DoublePoint{}, false
	}
	half := math.Sqrt(4*radius*radius-d*d) / 2.0
	midX := 0.5 * float64(c1.X+c2.X)
	midY := 0.5 * float64(c1.Y+c2.Y)
	first = DoublePoint{X: midX - dy*half/d, Y: midY + dx*half/d}
	second = DoublePoint{X: midX + dy*half/d, Y: midY - dx*half/d}
	return first, second, true
}

// line2CircleIntersect returns where the segment p1→p2 crosses the circle of the given radius
// about centre c. With clamp the crossings are restricted to the segment; without it, to the
// infinite line. When two crossings are returned the first is the one nearer p1. An empty slice
// means no (in-range) crossing.
func line2CircleIntersect(c clipper.IntPoint, radius float64, p1, p2 DoublePoint, clamp bool) []DoublePoint {
	dx := p2.X - p1.X
	dy := p2.Y - p1.Y
	lcx := p1.X - float64(c.X)
	lcy := p1.Y - float64(c.Y)
	a := dx*dx + dy*dy
	b := 2*dx*lcx + 2*dy*lcy
	cc := lcx*lcx + lcy*lcy - radius*radius
	disc := b*b - 4*a*cc
	if disc < 0 {
		return nil
	}
	sq := math.Sqrt(disc)
	t1 := (-b - sq) / (2 * a)
	t2 := (-b + sq) / (2 * a)
	var result []DoublePoint
	if (t1 >= 0.0 && t1 <= 1.0) || !clamp {
		result = append(result, DoublePoint{X: p1.X + t1*dx, Y: p1.Y + t1*dy})
	}
	if (t2 >= 0.0 && t2 <= 1.0) || !clamp {
		result = append(result, DoublePoint{X: p1.X + t2*dx, Y: p1.Y + t2*dy})
	}
	return result
}

// compute2DPolygonCentroid returns the area centroid of a closed polygon (the shoelace-weighted
// average of edge midpoints), as the solver uses to pick a starting reference inside a region.
func compute2DPolygonCentroid(vertices clipper.Path) clipper.IntPoint {
	var cx, cy, signedArea float64
	size := len(vertices)
	for i := 0; i < size; i++ {
		x0 := float64(vertices[i].X)
		y0 := float64(vertices[i].Y)
		x1 := float64(vertices[(i+1)%size].X)
		y1 := float64(vertices[(i+1)%size].Y)
		a := x0*y1 - x1*y0
		signedArea += a
		cx += (x0 + x1) * a
		cy += (y0 + y1) * a
	}
	signedArea *= 0.5
	cx /= 6.0 * signedArea
	cy /= 6.0 * signedArea
	return clipper.IntPoint{X: int64(cx), Y: int64(cy)}
}
