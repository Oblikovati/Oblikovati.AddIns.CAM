// SPDX-License-Identifier: GPL-2.0-only

package geom2d

import "sort"

// ClipOutside splits a path (a polyline; pass a closed loop with its first point repeated at the
// end to clip a ring) into the maximal sub-paths that lie OUTSIDE the keepout polygon. Each
// segment is cut where it crosses the keepout boundary, and only the portions whose midpoint is
// outside are kept. Used to route a clearing pass around islands without gouging them. A
// degenerate keepout (<3 points) returns the path unchanged.
func ClipOutside(path []Point2, keepout Polygon) [][]Point2 {
	if len(keepout) < 3 || len(path) < 2 {
		return [][]Point2{path}
	}
	var runs [][]Point2
	var cur []Point2
	flush := func() {
		if len(cur) >= 2 {
			runs = append(runs, cur)
		}
		cur = nil
	}
	for i := 0; i+1 < len(path); i++ {
		a, b := path[i], path[i+1]
		ts := append([]float64{0}, segCrossings(a, b, keepout)...)
		ts = append(ts, 1)
		for k := 0; k+1 < len(ts); k++ {
			t0, t1 := ts[k], ts[k+1]
			if keepout.Contains(lerp(a, b, (t0+t1)/2)) {
				flush() // this sub-segment is inside the island — break the run
				continue
			}
			if len(cur) == 0 {
				cur = append(cur, lerp(a, b, t0))
			}
			cur = append(cur, lerp(a, b, t1))
		}
	}
	flush()
	return runs
}

// ClipInside is the dual of ClipOutside: it splits a path into the maximal sub-paths that lie
// INSIDE the region polygon, cutting each segment where it crosses the region boundary and keeping
// the portions whose midpoint is inside. Used to fill a pocket interior with scanline rows. A
// degenerate region (<3 points) returns no runs (nothing is inside it).
func ClipInside(path []Point2, region Polygon) [][]Point2 {
	if len(region) < 3 || len(path) < 2 {
		return nil
	}
	var runs [][]Point2
	var cur []Point2
	flush := func() {
		if len(cur) >= 2 {
			runs = append(runs, cur)
		}
		cur = nil
	}
	for i := 0; i+1 < len(path); i++ {
		a, b := path[i], path[i+1]
		ts := append([]float64{0}, segCrossings(a, b, region)...)
		ts = append(ts, 1)
		for k := 0; k+1 < len(ts); k++ {
			t0, t1 := ts[k], ts[k+1]
			if !region.Contains(lerp(a, b, (t0+t1)/2)) {
				flush() // this sub-segment is outside the region — break the run
				continue
			}
			if len(cur) == 0 {
				cur = append(cur, lerp(a, b, t0))
			}
			cur = append(cur, lerp(a, b, t1))
		}
	}
	flush()
	return runs
}

// SegmentCrosses reports whether the segment a→b properly crosses any edge of the polygon. A
// segment that merely runs along (collinear with) an edge does not count as crossing — which is
// what lets a pocket link move travel along a wall without being treated as leaving the region.
func (p Polygon) SegmentCrosses(a, b Point2) bool {
	return len(segCrossings(a, b, p)) > 0
}

// segCrossings returns the sorted, de-duplicated parameter values t in (0,1) where the segment
// a→b crosses an edge of the polygon.
func segCrossings(a, b Point2, poly Polygon) []float64 {
	var ts []float64
	n := len(poly)
	for i := 0; i < n; i++ {
		if t, ok := segIntersectT(a, b, poly[i], poly[(i+1)%n]); ok {
			ts = append(ts, t)
		}
	}
	sort.Float64s(ts)
	return dedupFloats(ts)
}

// segIntersectT returns the parameter t along p→p2 at which it crosses q→q2, if they intersect
// within both segments' interiors. Parallel/degenerate segments report ok=false.
func segIntersectT(p, p2, q, q2 Point2) (float64, bool) {
	r := Point2{X: p2.X - p.X, Y: p2.Y - p.Y}
	s := Point2{X: q2.X - q.X, Y: q2.Y - q.Y}
	denom := r.X*s.Y - r.Y*s.X
	if denom < epsilon && denom > -epsilon {
		return 0, false
	}
	qp := Point2{X: q.X - p.X, Y: q.Y - p.Y}
	t := (qp.X*s.Y - qp.Y*s.X) / denom
	u := (qp.X*r.Y - qp.Y*r.X) / denom
	if t <= epsilon || t >= 1-epsilon || u <= epsilon || u >= 1-epsilon {
		return 0, false
	}
	return t, true
}

// lerp linearly interpolates between two points by t.
func lerp(a, b Point2, t float64) Point2 {
	return Point2{X: a.X + (b.X-a.X)*t, Y: a.Y + (b.Y-a.Y)*t}
}

// dedupFloats drops values within epsilon of the previous one (the slice must be sorted).
func dedupFloats(xs []float64) []float64 {
	if len(xs) == 0 {
		return xs
	}
	out := xs[:1]
	for _, x := range xs[1:] {
		if x-out[len(out)-1] > epsilon {
			out = append(out, x)
		}
	}
	return out
}
