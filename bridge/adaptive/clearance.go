// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

package adaptive

import (
	"math"

	"oblikovati.org/cam/bridge/clipper"
)

// isClearPath reports whether the tool can travel along tp (a link/rapid move) without touching
// uncut stock: the swept tool footprint (radius plus a safety clearance) is subtracted from the
// given cleared region and the move is clear only if almost no footprint falls outside it. Exact
// port of IsClearPath (cleared is explicit so the link machinery can test a working copy).
func (rp *regionProcessor) isClearPath(tp clipper.Path, cleared *clearedArea, safetyClearance float64) (bool, error) {
	toolShape, err := clipper.Offset(clipper.Paths{tp}, clipper.Round, clipper.OpenRound, float64(rp.s.toolRadiusScaled)+safetyClearance, 0, 0)
	if err != nil {
		return false, err
	}
	crossing, err := clipper.Subtract(toolShape, cleared.cleared())
	if err != nil {
		return false, err
	}
	collisionArea := 0.0
	for _, p := range crossing {
		collisionArea += math.Abs(clipper.Area(p))
	}
	return collisionArea < 1.0, nil
}

// isAllowedToCutThrough reports whether the tool may cut in a straight line from p1 to p2 without
// over-engaging: it walks the segment in steps and rejects it if any step cuts more than
// areaFactor times the optimal engagement, or if the tool would leave the region. A move shorter
// than half a step is treated as an insignificant cut and allowed. Exact port of
// IsAllowedToCutTrough (cleared is explicit; areaFactor defaults to 1.5 at the call sites).
func (rp *regionProcessor) isAllowedToCutThrough(p1, p2 clipper.IntPoint, cleared *clearedArea, areaFactor float64, skipBoundsCheck bool) (bool, error) {
	if !skipBoundsCheck && (!isPointWithinCutRegion(rp.toolBoundPaths, p2) || !isPointWithinCutRegion(rp.toolBoundPaths, p1)) {
		return false, nil // an endpoint is outside the region — not clear to cut
	}
	distance := distanceBetween(p1, p2)
	stepSize := math.Min(0.5*rp.s.stepOverScaled, 8*minStepClipper)
	if distance < stepSize/2 {
		return true, nil // not a significant cut
	}
	if distance < stepSize {
		areaFactor *= 2 // loosen for numeric instability at small distances
	}

	toolPos1 := p1
	steps := int64(distance/stepSize) + 1
	stepSize = distance / float64(steps)
	for i := int64(1); i <= steps; i++ {
		frac := float64(i) / float64(steps)
		toolPos2 := clipper.IntPoint{
			X: p1.X + int64(float64(p2.X-p1.X)*frac),
			Y: p1.Y + int64(float64(p2.Y-p1.Y)*frac),
		}
		area, _, err := rp.cutArea(toolPos1, toolPos2, cleared)
		if err != nil {
			return false, err
		}
		if area > areaFactor*stepSize*rp.s.optimalCutAreaPD {
			return false, nil // cutting above the allowed engagement
		}
		if !skipBoundsCheck && !isPointWithinCutRegion(rp.toolBoundPaths, toolPos2) {
			return false, nil
		}
		toolPos1 = toolPos2
	}
	return true, nil
}
