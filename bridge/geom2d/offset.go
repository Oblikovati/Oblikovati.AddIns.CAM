// SPDX-License-Identifier: GPL-2.0-only

package geom2d

import "math"

// Offset returns the polygon offset by dist with a miter (sharp-corner) join: positive dist
// grows the loop outward, negative shrinks it inward. The input is first normalised to CCW
// so the sign is well defined. ok is false when an inward offset has collapsed the loop
// (area driven to ~zero or the winding inverted) — the caller uses that to stop generating
// inner pocket rings.
//
// This is a miter offset (the JoinType=Miter case of FreeCAD's area engine). It is exact for
// convex polygons and correct for mildly concave ones; deeply concave shapes whose inward
// offset self-intersects are a known limitation (M2 documents it — robust arbitrary-polygon
// offsetting is the host's OffsetPlanarWire / a future Clipper-grade port).
func Offset(p Polygon, dist float64) (Polygon, bool) {
	if len(p) < 3 {
		return nil, false
	}
	ccw := p.EnsureCCW()
	n := len(ccw)
	out := make(Polygon, n)
	for j := 0; j < n; j++ {
		prev := ccw[(j-1+n)%n]
		cur := ccw[j]
		next := ccw[(j+1)%n]
		out[j] = offsetVertex(prev, cur, next, dist)
	}
	if out.SignedArea() <= epsilon || !edgesPreserved(ccw, out) {
		return nil, false
	}
	return out, true
}

// edgesPreserved reports whether every offset edge still points the same way as its original
// edge. When an inward offset over-collapses, opposite edges cross and an edge reverses
// direction even though the overall winding (and signed area) can stay positive — this
// catches that case (e.g. shrinking a square by more than half its side).
func edgesPreserved(orig, off Polygon) bool {
	n := len(orig)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		od := Point2{X: orig[j].X - orig[i].X, Y: orig[j].Y - orig[i].Y}
		nd := Point2{X: off[j].X - off[i].X, Y: off[j].Y - off[i].Y}
		if od.X*nd.X+od.Y*nd.Y <= 0 {
			return false
		}
	}
	return true
}

// offsetVertex computes the offset position of vertex cur, where the incoming edge is
// prev→cur and the outgoing edge is cur→next, each shifted outward by dist along its normal.
// The new vertex is the intersection of the two shifted edge lines (the miter point); for a
// straight/degenerate corner it falls back to the averaged-normal offset.
func offsetVertex(prev, cur, next Point2, dist float64) Point2 {
	n1, ok1 := outwardNormal(prev, cur)
	n2, ok2 := outwardNormal(cur, next)
	if !ok1 && !ok2 {
		return cur
	}
	if !ok1 {
		return shift(cur, n2, dist)
	}
	if !ok2 {
		return shift(cur, n1, dist)
	}
	q1 := shift(cur, n1, dist)      // point on the shifted incoming edge
	q2 := shift(cur, n2, dist)      // point on the shifted outgoing edge
	d1 := Point2{X: -n1.Y, Y: n1.X} // direction along the incoming edge
	d2 := Point2{X: -n2.Y, Y: n2.X} // direction along the outgoing edge
	if pt, ok := intersect(q1, d1, q2, d2); ok {
		return pt
	}
	// Parallel edges (straight-through or reversal): offset along the averaged normal.
	avg := Point2{X: n1.X + n2.X, Y: n1.Y + n2.Y}
	if l := math.Hypot(avg.X, avg.Y); l > epsilon {
		return shift(cur, Point2{X: avg.X / l, Y: avg.Y / l}, dist)
	}
	return shift(cur, n1, dist)
}

// outwardNormal returns the unit outward normal of the directed edge a→b for a CCW polygon
// (the right-hand normal (dir.Y, -dir.X)). ok is false for a zero-length edge.
func outwardNormal(a, b Point2) (Point2, bool) {
	dx, dy := b.X-a.X, b.Y-a.Y
	l := math.Hypot(dx, dy)
	if l < epsilon {
		return Point2{}, false
	}
	return Point2{X: dy / l, Y: -dx / l}, true
}

// shift moves a point by dist along the unit normal n.
func shift(p, n Point2, dist float64) Point2 {
	return Point2{X: p.X + n.X*dist, Y: p.Y + n.Y*dist}
}

// intersect returns the intersection of the lines (p1 + t·d1) and (p2 + s·d2). ok is false
// when the lines are parallel.
func intersect(p1, d1, p2, d2 Point2) (Point2, bool) {
	denom := d1.X*d2.Y - d1.Y*d2.X
	if math.Abs(denom) < epsilon {
		return Point2{}, false
	}
	t := ((p2.X-p1.X)*d2.Y - (p2.Y-p1.Y)*d2.X) / denom
	return Point2{X: p1.X + t*d1.X, Y: p1.Y + t*d1.Y}, true
}
