// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

// The engage-point search picks where the tool should next plunge/enter to start a pass. It drives
// the clipping engine (offsets, open-path-area intersection) and the lead/link solvers, so it is
// cgo-only.

package adaptive

import (
	"math"
	"sort"

	"oblikovati.org/cam/bridge/clipper"
)

// engageBuffer absorbs the integer rounding of the two opposed offsets (−toolRadius then +) used to
// find the thin band along the cleared-area border where the tool can start a new pass.
const engageBuffer = 2.0

// engageCandidate is a possible start location for the next pass: a point in the band along the
// cleared border, the cut direction there, and the straight-line cost of reaching it from the
// previous tool position.
type engageCandidate struct {
	point clipper.IntPoint
	dir   DoublePoint
	cost  float64
}

// engageResult is the chosen engage point together with the resolved link path that reaches it.
type engageResult struct {
	pos  clipper.IntPoint
	dir  DoublePoint
	link []TPath
}

// getEngagePoint chooses where the next pass begins: it sizes how far into the material the first
// engagement should protrude (from the target cut area), finds candidate start points along the
// cleared border, picks the one with the cheapest resolved link, and folds that link's cutting
// moves into the cleared area. Returns nil when no engagement is left. Exact port of getEngagePoint.
func (rp *regionProcessor) getEngagePoint(prevPos *clipper.IntPoint, toolBoundPaths, tbpMinus clipper.Paths) (*engageResult, error) {
	// how far into the material the first engagement should reach, from the target cut area:
	// a circular segment of area A on a radius-R tool subtends theta ≈ (12A/R²)^(1/3); the
	// protrusion is R(1−cos(theta/2)), capped at the finishing thickness.
	targetArea := rp.s.optimalCutAreaPD * minStepClipper
	r := float64(rp.s.toolRadiusScaled)
	theta := math.Pow(12*targetArea/r/r, 1.0/3.0)
	protrusion := r - math.Cos(theta/2)*r
	engagementProtrusion := int64(math.Min(protrusion, rp.s.stepOverScaled*finishingThicknessScale))

	candidates, err := rp.findEngageCandidates(prevPos, engagementProtrusion, tbpMinus)
	if err != nil {
		return nil, err
	}
	result, err := rp.bestEngagePoint(prevPos, candidates, toolBoundPaths)
	if err != nil {
		return nil, err
	}
	if result != nil {
		for _, lp := range result.link {
			if lp.Motion == MotionCutting {
				if err := rp.cleared.expandCleared(rp.scaleToClipperPath(lp.Pts)); err != nil {
					return nil, err
				}
			}
		}
	}
	return result, nil
}

// findEngageCandidates builds the band along the cleared-area border (offset inward by the tool
// radius, then back out by the engagement protrusion), clips each border ring to the tool bounds,
// and walks each resulting open path sampling start directions. Exact port of the candidate-finding
// half of _getEngagePoint.
func (rp *regionProcessor) findEngageCandidates(prevPos *clipper.IntPoint, engagementProtrusion int64, tbpMinus clipper.Paths) ([]engageCandidate, error) {
	preEngage, err := clipper.Offset(rp.cleared.cleared(), clipper.Round, clipper.ClosedPolygon, -(float64(rp.s.toolRadiusScaled) + engageBuffer), 0, 0)
	if err != nil {
		return nil, err
	}
	engagePaths, err := clipper.Offset(preEngage, clipper.Round, clipper.ClosedPolygon, engageBuffer+float64(engagementProtrusion), 0, 0)
	if err != nil {
		return nil, err
	}
	var candidates []engageCandidate
	for _, engagePath := range engagePaths {
		// rotate so the ring starts at the point closest to prevPos: if the ring is not clipped any
		// point on it is a valid start, but we want to test the nearest one first
		rotated := rotateToClosest(engagePath, prevPos)
		openPaths, err := clipper.PathIntersectArea(rotated, tbpMinus)
		if err != nil {
			return nil, err
		}
		for _, open := range openPaths {
			cand, found, err := rp.firstEngageAlong(open, prevPos)
			if err != nil {
				return nil, err
			}
			if found {
				candidates = append(candidates, cand)
			}
		}
	}
	return candidates, nil
}

// rotateToClosest returns engagePath rotated to begin at the vertex nearest prevPos (or unchanged
// when there is no previous position).
func rotateToClosest(engagePath clipper.Path, prevPos *clipper.IntPoint) clipper.Path {
	if prevPos == nil {
		return engagePath
	}
	iClosest := 0
	dsqClosest := math.MaxFloat64
	for i, p := range engagePath {
		if dsq := distanceSqrd(*prevPos, p); dsq < dsqClosest {
			dsqClosest = dsq
			iClosest = i
		}
	}
	rotated := make(clipper.Path, 0, len(engagePath))
	for i := range engagePath {
		rotated = append(rotated, engagePath[(i+iClosest)%len(engagePath)])
	}
	return rotated
}

// firstEngageAlong steps along an open border path (first sample at its start, then every
// minStepClipper) and returns the first sampled point where initToolDir finds a valid cut
// direction. Exact port of the per-open-path walk in _getEngagePoint.
func (rp *regionProcessor) firstEngageAlong(open clipper.Path, prevPos *clipper.IntPoint) (engageCandidate, bool, error) {
	dToGo := 0.0 // first sample is the start point
	seg := 0
	segD := 0.0
	for seg < len(open)-1 {
		p, segDir, ok := advanceAlong(open, &seg, &segD, &dToGo)
		if !ok {
			break // ran off the end on a degenerate span
		}
		dir, found, err := rp.initToolDir(p, segDir)
		if err != nil {
			return engageCandidate{}, false, err
		}
		if found {
			return engageCandidate{point: p, dir: dir, cost: engageCost(prevPos, p, rp.s.scaleFactor)}, true, nil
		}
		dToGo = minStepClipper // subsequent samples step by minStepClipper
	}
	return engageCandidate{}, false, nil
}

// advanceAlong consumes dToGo distance along the open path from (seg,segD), returning the landed
// point and the direction of the segment it lies on. It either interpolates within the current
// segment or steps to the next vertex, mirroring the upstream inner do-while. ok is false if it
// walked off the end past degenerate (zero-length) spans. Exact port of the inner stepping loop.
func advanceAlong(open clipper.Path, seg *int, segD, dToGo *float64) (clipper.IntPoint, DoublePoint, bool) {
	var p clipper.IntPoint
	var segDir DoublePoint
	for {
		p1 := open[*seg]
		p2 := open[*seg+1]
		segLen := math.Sqrt(distanceSqrd(p1, p2))
		if segLen < numericTolerance {
			*seg++
			*segD = 0
			if *seg >= len(open)-1 {
				return clipper.IntPoint{}, DoublePoint{}, false
			}
			continue
		}
		segDir = DoublePoint{X: float64(p2.X-p1.X) / segLen, Y: float64(p2.Y-p1.Y) / segLen}
		if segLen-*segD > *dToGo {
			*segD += *dToGo
			*dToGo = 0
			interp := *segD / segLen
			p = clipper.IntPoint{X: int64(float64(p2.X)*interp + float64(p1.X)*(1-interp)), Y: int64(float64(p2.Y)*interp + float64(p1.Y)*(1-interp))}
		} else {
			*dToGo -= segLen - *segD
			*segD = 0
			*seg++
			p = p2
		}
		if !(*dToGo > 0 && *seg < len(open)-1) {
			return p, segDir, true
		}
	}
}

// bestEngagePoint sorts the candidates by straight-line cost, then resolves a link path to each in
// turn (pruning once the straight-line cost can no longer beat the best link), keeping the one whose
// link is cheapest — retraction links are heavily penalised. Exact port of the selection half of
// _getEngagePoint.
func (rp *regionProcessor) bestEngagePoint(prevPos *clipper.IntPoint, candidates []engageCandidate, toolBoundPaths clipper.Paths) (*engageResult, error) {
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].cost < candidates[j].cost })

	bestCost := math.MaxFloat64
	var best *engageResult
	for _, ep := range candidates {
		if ep.cost >= bestCost {
			continue // sorted ascending: no later candidate can beat the best link cost
		}
		link, ok, err := rp.findLinkPath(prevPos, ep.point, ep.dir, rp.cleared, toolBoundPaths)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if cost := linkCostMM(prevPos, link, rp.s.scaleFactor); cost < bestCost {
			bestCost = cost
			best = &engageResult{pos: ep.point, dir: ep.dir, link: link}
		}
	}
	return best, nil
}

// engageCost is the straight-line distance (mm) from prevPos to an engage point (0 with no prevPos).
func engageCost(prevPos *clipper.IntPoint, engagePoint clipper.IntPoint, scaleFactor int64) float64 {
	if prevPos == nil {
		return 0
	}
	return math.Sqrt(distanceSqrd(*prevPos, engagePoint)) / float64(scaleFactor)
}

// linkCostMM totals the travelled length (mm) of a link path, adding a large penalty for each
// not-clear (retracting) segment so keep-tool-down links win. Exact port of the cost_mm loop.
func linkCostMM(prevPos *clipper.IntPoint, link []TPath, scaleFactor int64) float64 {
	cost := 0.0
	var prev *DoublePoint
	if prevPos != nil {
		p := DoublePoint{X: float64(prevPos.X) / float64(scaleFactor), Y: float64(prevPos.Y) / float64(scaleFactor)}
		prev = &p
	}
	for _, tp := range link {
		if tp.Motion == MotionLinkNotClear {
			cost += 10000 // prioritise links that don't require retraction
		}
		for _, cur := range tp.Pts {
			if prev != nil {
				dx := cur.X - prev.X
				dy := cur.Y - prev.Y
				cost += math.Sqrt(dx*dx + dy*dy)
			}
			c := cur
			prev = &c
		}
	}
	return cost
}

// scaleToClipperPath converts a millimetre DPath back into the scaled integer plane.
func (rp *regionProcessor) scaleToClipperPath(pts DPath) clipper.Path {
	out := make(clipper.Path, len(pts))
	sf := float64(rp.s.scaleFactor)
	for i, p := range pts {
		out[i] = clipper.IntPoint{X: int64(p.X * sf), Y: int64(p.Y * sf)}
	}
	return out
}
