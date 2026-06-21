// SPDX-License-Identifier: GPL-2.0-only

package clipper

// CleanPolygon strips vertices that sit within `distance` (in integer plane units) of a
// neighbour, and vertices that lie within `distance` of the line through their two
// neighbours (near-collinear "spikes"). The clearing solver runs it on every boolean
// result to drop the near-duplicate / near-collinear points the integer rounding leaves
// behind, which otherwise make the engagement geometry noisy. `distance` defaults to about
// sqrt(2) in the engine. Returns a new path; the input is not modified.
//
// This is a faithful port of ClipperLib's CleanPolygon, which walks a circular doubly
// linked list of points and splices out the rejects; the index-based prev/next arrays here
// stand in for the C++ OutPt node pointers, so the splice/exclude order — and therefore the
// output — matches exactly.
func CleanPolygon(in Path, distance float64) Path {
	size := len(in)
	if size == 0 {
		return Path{}
	}
	next := make([]int, size)
	prev := make([]int, size)
	idx := make([]int, size) // 0 = unvisited (Clipper's OutPt.Idx)
	for i := 0; i < size; i++ {
		next[i] = (i + 1) % size
		prev[next[i]] = i
	}
	// exclude splices node op out of the list and returns its predecessor, mirroring
	// ClipperLib's ExcludeOp (which also clears the predecessor's visited flag so it is
	// re-examined).
	exclude := func(op int) int {
		p := prev[op]
		next[p] = next[op]
		prev[next[op]] = p
		idx[p] = 0
		return p
	}

	distSqrd := distance * distance
	op := 0
	for idx[op] == 0 && next[op] != prev[op] {
		switch {
		case pointsAreClose(in[op], in[prev[op]], distSqrd):
			op = exclude(op)
			size--
		case pointsAreClose(in[prev[op]], in[next[op]], distSqrd):
			exclude(next[op])
			op = exclude(op)
			size -= 2
		case slopesNearCollinear(in[prev[op]], in[op], in[next[op]], distSqrd):
			op = exclude(op)
			size--
		default:
			idx[op] = 1
			op = next[op]
		}
	}
	if size < 3 {
		size = 0
	}
	out := make(Path, 0, size)
	for i := 0; i < size; i++ {
		out = append(out, in[op])
		op = next[op]
	}
	return out
}

// CleanPolygons cleans every path in the set with the same distance, returning a new set.
func CleanPolygons(in Paths, distance float64) Paths {
	out := make(Paths, len(in))
	for i := range in {
		out[i] = CleanPolygon(in[i], distance)
	}
	return out
}

// pointsAreClose reports whether two points are within sqrt(distSqrd) of each other.
func pointsAreClose(p1, p2 IntPoint, distSqrd float64) bool {
	dx := float64(p1.X - p2.X)
	dy := float64(p1.Y - p2.Y)
	return dx*dx+dy*dy <= distSqrd
}

// distanceFromLineSqrd returns the squared perpendicular distance of pt from the line
// through ln1 and ln2 (general-form line equation), as in ClipperLib.
func distanceFromLineSqrd(pt, ln1, ln2 IntPoint) float64 {
	a := float64(ln1.Y - ln2.Y)
	b := float64(ln2.X - ln1.X)
	c := a*float64(ln1.X) + b*float64(ln1.Y)
	c = a*float64(pt.X) + b*float64(pt.Y) - c
	return (c * c) / (a*a + b*b)
}

// slopesNearCollinear reports whether the three points are near-collinear within distSqrd.
// It measures the distance from whichever point lies geometrically between the other two,
// which makes it more likely to catch spikes — the exact heuristic ClipperLib uses.
func slopesNearCollinear(pt1, pt2, pt3 IntPoint, distSqrd float64) bool {
	if absI64(pt1.X-pt2.X) > absI64(pt1.Y-pt2.Y) {
		switch {
		case (pt1.X > pt2.X) == (pt1.X < pt3.X):
			return distanceFromLineSqrd(pt1, pt2, pt3) < distSqrd
		case (pt2.X > pt1.X) == (pt2.X < pt3.X):
			return distanceFromLineSqrd(pt2, pt1, pt3) < distSqrd
		default:
			return distanceFromLineSqrd(pt3, pt1, pt2) < distSqrd
		}
	}
	switch {
	case (pt1.Y > pt2.Y) == (pt1.Y < pt3.Y):
		return distanceFromLineSqrd(pt1, pt2, pt3) < distSqrd
	case (pt2.Y > pt1.Y) == (pt2.Y < pt3.Y):
		return distanceFromLineSqrd(pt2, pt1, pt3) < distSqrd
	default:
		return distanceFromLineSqrd(pt3, pt1, pt2) < distSqrd
	}
}

// absI64 is the int64 absolute value used by the near-collinear test.
func absI64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
