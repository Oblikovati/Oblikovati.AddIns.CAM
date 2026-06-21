// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import "oblikovati.org/cam/bridge/clipper"

// isPointWithinCutRegion reports whether a point lies in the machinable region defined by the
// tool-bound paths under the even-odd rule: a point inside the outer boundary but inside an odd
// number of nested paths (i.e. in a hole) is outside the region. The solver uses it to keep each
// stepped tool position within bounds. Exact port of IsPointWithinCutRegion.
func isPointWithinCutRegion(toolBoundPaths clipper.Paths, point clipper.IntPoint) bool {
	inside := false
	for _, p := range toolBoundPaths {
		if clipper.PointInPolygon(point, p) != 0 {
			inside = !inside
		}
	}
	return inside
}

// conventionalFraction is the share of a cut that lies on the conventional-milling side (0 when
// nothing was cut). The cutting loop rejects a move whose conventional share is too high.
func conventionalFraction(area, conventional float64) float64 {
	if area == 0 {
		return 0
	}
	return conventional / area
}
