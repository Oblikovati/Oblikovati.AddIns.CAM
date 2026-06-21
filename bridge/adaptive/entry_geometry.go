// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import "oblikovati.org/cam/bridge/clipper"

// The pure (engine-free) helpers of the entry search: bounding box, the symmetry-break quadrant,
// and choosing the deepest interior candidate from a set of inward offsets.

// pathsBounds is the bounding box of every point across the path set (a zero box when empty).
func pathsBounds(paths clipper.Paths) boundBox {
	bb := boundBox{}
	first := true
	for _, p := range paths {
		for _, pt := range p {
			if first {
				bb = newBoundBoxPoint(pt)
				first = false
			} else {
				bb.addPoint(pt)
			}
		}
	}
	return bb
}

// hasPoints reports whether any path in the set has a vertex (an all-empty set must not be fed to
// the clipping engine — an intersection with no clip operand is an error there).
func hasPoints(paths clipper.Paths) bool {
	for _, p := range paths {
		if len(p) > 0 {
			return true
		}
	}
	return false
}

// quadrantRect is the bottom-left quadrant of a bounding box, used to break the symmetry of a
// region whose centre fell on a hole. Exact port of the rectangle built in FindEntryPoint.
func quadrantRect(bb boundBox) clipper.Path {
	midX := (bb.minX + bb.maxX) / 2
	midY := (bb.minY + bb.maxY) / 2
	return clipper.Path{
		{X: bb.minX, Y: bb.maxY},
		{X: bb.minX, Y: midY},
		{X: midX, Y: midY},
		{X: midX, Y: bb.maxY},
	}
}

// pickDeepestCentroid takes the centroid of the first non-empty offset in lastValid (the deepest
// inward offset of the region) as the entry candidate, then rejects it if it landed outside the
// outer boundary (checkPaths[0]) or inside a hole (checkPaths[1:]). Returns the candidate and
// whether it is a valid interior point.
func pickDeepestCentroid(lastValid, checkPaths clipper.Paths) (clipper.IntPoint, bool) {
	var entry clipper.IntPoint
	found := false
	for _, p := range lastValid {
		if len(p) > 0 {
			entry = compute2DPolygonCentroid(p)
			found = true
			break
		}
	}
	if found {
		for j, cp := range checkPaths {
			pip := clipper.PointInPolygon(entry, cp)
			if (j == 0 && pip == 0) || (j > 0 && pip != 0) {
				return entry, false
			}
		}
	}
	return entry, found
}
