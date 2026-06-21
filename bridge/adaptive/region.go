// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

// The engagement cutting loop drives the clipping engine every step (cut-area queries, cleared-area
// expansion), so it is built only with cgo; the non-cgo adaptive path falls back to the simpler
// spiral. The pure predicates it uses live in the untagged region_geometry.go.

package adaptive

import (
	"math"

	"oblikovati.org/cam/bridge/clipper"
)

// maxIterations bounds the per-step angle search; areaErrorFactor is how tightly the cut area must
// match the target (5%); conventionalCutoff is the fraction of the cut on the conventional side
// above which a move is rejected as conventional (not climb) milling.
const (
	maxIterations       = 30
	areaErrorFactor     = 0.05
	conventionalCutoff  = 0.51
	rotateBackLimit     = 180
	rotateBackIncrement = math.Pi / 90
)

// iterateNextStepOutput is one step decision: where the tool should move next, how much it cuts,
// and whether the search failed (cut too little, went conventional, or could not be kept inside
// the region).
type iterateNextStepOutput struct {
	newToolPos        clipper.IntPoint
	newToolDir        DoublePoint
	area              float64
	errorFraction     float64
	iterationAngle    float64
	hasIterationAngle bool // whether a valid step angle was recorded (upstream optional<double>)
	failed            bool
	tooManyIterations bool
}

// regionProcessor holds the mutable per-region state of the cutting loop — the bounds, the
// growing cleared-area model, the engagement target, and the predictors (recent angles, the
// interpolation search) — so the step-decision methods can be expressed as in the upstream
// closures. One processor clears one connected region.
type regionProcessor struct {
	s              *solver
	toolBoundPaths clipper.Paths // region the tool centre may occupy
	boundPaths     clipper.Paths // true boundary
	cleared        *clearedArea
	entryPoint     clipper.IntPoint
	output         *Output

	angle             float64       // last deflection angle (seeds the step-size schedule)
	angleHistory      []float64     // recent angles, for prediction
	stepScaled        int64         // current step length (scaled)
	toClearPath       clipper.Path  // accumulated cut path pending an ExpandCleared
	lastExpandToolDir DoublePoint   // tool direction at the last cleared-area expansion
	interp            interpolation // the per-step angle search
	overCutCount      int

	// pass-loop state (set up by clearRegion / runPasses), kept here so the orchestration methods
	// read like the upstream closures
	tbpMinus          clipper.Paths // tool bound shrunk by 2, the engage search clips against it
	toolPos           clipper.IntPoint
	toolDir           DoublePoint
	gyro              []DoublePoint // recent tool directions, averaged to smooth the path
	clearedBeforePass *clearedArea  // snapshot to measure each pass's newly-cleared area
	linkPath          []TPath       // the link/lead-in feeding the current pass
	helixRadiusScaled int64
}

// cutArea measures the freshly cut area for a move from c1 to c2 against the given cleared-area
// model, integrating with calcCutArea. It reproduces the bounding-window selection CalcCutArea does
// internally: a far move looks around c2, a near move around c1 with a window enlarged by the move
// distance. The cleared area is passed explicitly because the lead/link machinery measures against
// a local working copy, not the region's accumulating one.
func (rp *regionProcessor) cutArea(c1, c2 clipper.IntPoint, cleared *clearedArea) (area, conventional float64, err error) {
	dist := distanceBetween(c1, c2)
	center := c1
	delta := rp.s.toolRadiusScaled + int64(dist) + 4
	if dist > float64(2*rp.s.toolRadiusScaled) {
		center = c2
		delta = rp.s.toolRadiusScaled + 4
	}
	bounded, err := cleared.boundedClearedAreaClipped(center, delta)
	if err != nil {
		return 0, 0, err
	}
	area, conventional = calcCutArea(c1, c2, rp.s.toolRadiusScaled, bounded)
	return area, conventional, nil
}

// stepSizeFor sets stepScaled for this step: the minimum near the boundary or the engage point, a
// larger step on gentle curves (inversely proportional to the deflection angle), clamped to a
// stable range. Exact port of the step-size block.
func (rp *regionProcessor) stepSizeFor(distanceToBoundary, distanceToEngage float64) {
	slowDown := math.Max(float64(rp.s.toolRadiusScaled)/4, minStepClipper*8)
	switch {
	case distanceToBoundary < slowDown || distanceToEngage < slowDown:
		rp.stepScaled = int64(minStepClipper)
	case math.Abs(rp.angle) > numericTolerance:
		rp.stepScaled = int64(minStepClipper / math.Abs(rp.angle))
	default:
		rp.stepScaled = int64(minStepClipper * 8)
	}
	maxStep := min64(rp.s.toolRadiusScaled/4, int64(minStepClipper*8))
	if rp.stepScaled > maxStep {
		rp.stepScaled = maxStep
	}
	if rp.stepScaled < int64(minStepClipper) {
		rp.stepScaled = int64(minStepClipper)
	}
}

// iterateNextStep searches for the next tool position that holds the target engagement starting
// from toolPos heading toolDir: it sets the step size, then sweeps the deflection angle (predicted,
// max-engage, interpolated…) probing the cut area each time, recovers by expanding the cleared area
// and steering toward nearby uncut stock if nothing cuts, rotates the result back inside the region
// if it strayed, and flags failure when it cannot hold a valid climb-cut. Exact port of the
// iterateNextStep lambda.
func (rp *regionProcessor) iterateNextStep(toolPos clipper.IntPoint, toolDir DoublePoint, warnRotate bool) (iterateNextStepOutput, error) {
	var out iterateNextStepOutput
	s := rp.s

	closest := distancePointToPathsSqrd(rp.toolBoundPaths, toolPos)
	distanceToBoundary := math.Sqrt(closest.distSqrd)
	boundaryDir := getPathDirectionV(rp.toolBoundPaths[closest.pathIndex], closest.segmentIndex)
	distanceToEngage := distanceBetween(toolPos, rp.entryPoint)

	targetAreaPD := s.optimalCutAreaPD
	rp.stepSizeFor(distanceToBoundary, distanceToEngage)

	predictedAngle := averageDV(rp.angleHistory)
	maxError := areaErrorFactor * s.optimalCutAreaPD
	errorFraction := 1.0
	area := 0.0
	isConventional := false
	areaPD := 0.0
	rp.interp.clear()

	var pointNotInterp, foundArea bool
	var newToolPos clipper.IntPoint
	var newToolDir DoublePoint

search:
	for iteration := 0; iteration < maxIterations; iteration++ {
		switch {
		case iteration == 0:
			rp.angle = predictedAngle
			pointNotInterp = true
		case iteration == 1:
			rp.angle = interpMinAngle // max engage
			pointNotInterp = true
		case iteration == 2:
			if rp.interp.bothSides() {
				rp.angle = rp.interp.interpolateAngle()
				pointNotInterp = false
			} else {
				rp.angle = interpMaxAngle // min engage
				pointNotInterp = true
			}
		case iteration == 3 && !foundArea:
			cont, err := rp.recoverTowardUncleared(toolPos, toolDir, &rp.angle)
			if err != nil {
				return out, err
			}
			if cont {
				continue
			}
		case !foundArea:
			// nothing cut and no recovery possible: nothing will cut, stop searching
			rp.angle, area, areaPD = 0, 0, 0
			break search
		default:
			rp.angle = rp.interp.interpolateAngle()
			pointNotInterp = false
		}
		rp.angle = rp.interp.clampAngle(rp.angle)
		newToolDir = rotateVec(toolDir, rp.angle)
		newToolPos = rp.advance(toolPos, newToolDir)

		if repeated, brk := rp.handleRepeatPoint(toolDir, rp.angle, pointNotInterp, newToolPos, &newToolDir, &newToolPos, &area, &areaPD, &isConventional, targetAreaPD, &out); repeated {
			if brk {
				break
			}
			continue
		}

		a, conventionalArea, err := rp.cutArea(toolPos, newToolPos, rp.cleared)
		if err != nil {
			return out, err
		}
		area = a
		isConventional = conventionalFraction(area, conventionalArea) >= conventionalCutoff
		if area > 0 {
			foundArea = true
		}
		areaPD = area / float64(rp.stepScaled)
		errVal := areaPD - targetAreaPD
		errorFraction = math.Abs(errVal / s.optimalCutAreaPD)
		rp.interp.addPoint(errVal, rp.angle, newToolPos, pointNotInterp, isConventional)
		if math.Abs(errVal) < maxError && !isConventional {
			out.iterationAngle = rp.angle
			out.hasIterationAngle = true
			break
		}
		if iteration == maxIterations-1 {
			out.tooManyIterations = true
		}
	}

	return rp.finishStep(out, toolPos, &area, &areaPD, &errorFraction, &isConventional, boundaryDir, newToolDir, newToolPos, warnRotate)
}

// advance steps the tool one stepScaled along dir from p.
func (rp *regionProcessor) advance(p clipper.IntPoint, dir DoublePoint) clipper.IntPoint {
	return clipper.IntPoint{
		X: p.X + int64(dir.X*float64(rp.stepScaled)),
		Y: p.Y + int64(dir.Y*float64(rp.stepScaled)),
	}
}

// recoverTowardUncleared (iteration 3) expands the cleared area by what has been cut so far, then
// looks in a forward ±45° wedge for the nearest still-uncleared stock and aims the next probe at
// it. Returns cont=true when the wedge is already clear (the caller should continue the search).
func (rp *regionProcessor) recoverTowardUncleared(toolPos clipper.IntPoint, toolDir DoublePoint, angle *float64) (bool, error) {
	if err := rp.cleared.expandCleared(rp.toClearPath); err != nil {
		return false, err
	}
	rp.toClearPath = nil
	rp.lastExpandToolDir = toolDir

	dist := float64(rp.stepScaled+rp.s.toolRadiusScaled) * 1.5 // 1.5 > sqrt(2): wedge contains all possible steps
	left := rotateVec(toolDir, -math.Pi/4)
	right := rotateVec(toolDir, math.Pi/4)
	triangle := clipper.Path{
		toolPos,
		{X: toolPos.X + int64(right.X*dist), Y: toolPos.Y + int64(right.Y*dist)},
		{X: toolPos.X + int64(left.X*dist), Y: toolPos.Y + int64(left.Y*dist)},
	}
	uncleared, err := clipper.Subtract(clipper.Paths{triangle}, rp.cleared.cleared())
	if err != nil {
		return false, err
	}
	if len(uncleared) == 0 {
		return true, nil
	}
	near := distancePointToPathsSqrd(uncleared, toolPos)
	dy := float64(near.point.Y - toolPos.Y)
	dx := float64(near.point.X - toolPos.X)
	length := math.Sqrt(dx*dx + dy*dy)
	*angle = math.Asin((dy*toolDir.X - dx*toolDir.Y) / length)
	return false, nil
}

// handleRepeatPoint deals with a candidate position the integer rounding has already produced: it
// refreshes the matching interpolation sample's angle, and when the interpolation has narrowed to
// two adjacent integer points it selects the better of them and ends the search. Returns whether
// the point repeated and, if so, whether to break (vs continue). Exact port of the intRepeat block.
func (rp *regionProcessor) handleRepeatPoint(toolDir DoublePoint, angle float64, pointNotInterp bool, newToolPos clipper.IntPoint,
	newToolDir *DoublePoint, posOut *clipper.IntPoint, area, areaPD *float64, isConventional *bool, targetAreaPD float64, out *iterateNextStepOutput) (repeated, brk bool) {
	intRepeat := false
	if rp.interp.min != nil && newToolPos == rp.interp.min.point {
		rp.interp.min = &interpItem{angle: angle, point: newToolPos, errorVal: rp.interp.min.errorVal, isConventional: rp.interp.min.isConventional}
		intRepeat = true
	}
	if rp.interp.max != nil && newToolPos == rp.interp.max.point {
		rp.interp.max = &interpItem{angle: angle, point: newToolPos, errorVal: rp.interp.max.errorVal, isConventional: rp.interp.max.isConventional}
		intRepeat = true
	}
	if !intRepeat {
		return false, false
	}
	if rp.interp.min != nil && rp.interp.max != nil &&
		absI64(rp.interp.min.point.X-rp.interp.max.point.X) <= 1 &&
		absI64(rp.interp.min.point.Y-rp.interp.max.point.Y) <= 1 {
		if pointNotInterp {
			return true, false // adjacent only while probing the ends — keep searching
		}
		pick := rp.interp.max
		if rp.interp.min.isConventional != rp.interp.max.isConventional {
			if !rp.interp.min.isConventional {
				pick = rp.interp.min
			}
		} else if math.Abs(rp.interp.min.errorVal) < math.Abs(rp.interp.max.errorVal) {
			pick = rp.interp.min
		}
		*newToolDir = rotateVec(toolDir, pick.angle)
		*posOut = pick.point
		*isConventional = pick.isConventional
		*areaPD = pick.errorVal + targetAreaPD
		*area = *areaPD * float64(rp.stepScaled)
		out.iterationAngle = angle
		out.hasIterationAngle = true
		return true, true
	}
	return true, false
}

// finishStep applies the post-search checks: rotate the chosen position back inside the region if
// it strayed (recomputing the area), flag an over-cut, and roll up the failure conditions
// (conventional or near-zero area). Exact port of the tail of iterateNextStep.
func (rp *regionProcessor) finishStep(out iterateNextStepOutput, toolPos clipper.IntPoint, area, areaPD, errorFraction *float64, isConventional *bool,
	boundaryDir, newToolDir DoublePoint, newToolPos clipper.IntPoint, warnRotate bool) (iterateNextStepOutput, error) {
	s := rp.s
	if *area > 0 {
		recalc, nd, np, failed := rp.rotateBackIntoRegion(toolPos, boundaryDir, newToolDir, newToolPos)
		newToolDir, newToolPos = nd, np
		if failed {
			_ = warnRotate // DEV_MODE-only warning omitted
			out.failed = true
		}
		if recalc {
			a, conventionalArea, err := rp.cutArea(toolPos, newToolPos, rp.cleared)
			if err != nil {
				return out, err
			}
			*area = a
			*areaPD = a / float64(rp.stepScaled)
			*errorFraction = math.Abs((*areaPD - s.optimalCutAreaPD) / s.optimalCutAreaPD)
			*isConventional = conventionalFraction(a, conventionalArea) >= conventionalCutoff
		}
		if *area > float64(rp.stepScaled)*s.optimalCutAreaPD && *areaPD > 2*s.optimalCutAreaPD {
			rp.overCutCount++
			out.failed = true
		}
	}
	out.area = *area
	out.failed = out.failed || *isConventional || *area < 1
	out.newToolPos = newToolPos
	out.newToolDir = newToolDir
	out.errorFraction = *errorFraction
	return out, nil
}

// rotateBackIntoRegion turns the candidate move toward the boundary tangent until it lands back
// inside the region (up to 180 one-degree steps). Returns whether it had to move, the adjusted
// direction/position, and whether it gave up.
func (rp *regionProcessor) rotateBackIntoRegion(toolPos clipper.IntPoint, boundaryDir, newToolDir DoublePoint, newToolPos clipper.IntPoint) (recalc bool, dir DoublePoint, pos clipper.IntPoint, failed bool) {
	delta := math.Atan2(boundaryDir.Y, boundaryDir.X) - math.Atan2(newToolDir.Y, newToolDir.X)
	if delta > math.Pi {
		delta -= 2 * math.Pi
	}
	if delta < -math.Pi {
		delta += 2 * math.Pi
	}
	increment := rotateBackIncrement
	if delta <= 0 {
		increment = -rotateBackIncrement
	}
	rotateStep := 0
	for !isPointWithinCutRegion(rp.toolBoundPaths, newToolPos) && rotateStep < rotateBackLimit {
		rotateStep++
		recalc = true
		newToolDir = rotateVec(newToolDir, increment)
		newToolPos = rp.advance(toolPos, newToolDir)
	}
	return recalc, newToolDir, newToolPos, rotateStep >= rotateBackLimit
}

// initToolDir picks the starting cut direction at the entry: it probes the base direction and its
// three perpendiculars, keeping the non-failing one with the lowest engagement error. If none cut
// (the tool is islanded in already-cleared stock), it heads toward the nearest cleared region's
// centroid. Returns the direction and whether one was found. Exact port of initToolDir.
func (rp *regionProcessor) initToolDir(toolPos clipper.IntPoint, baseDir DoublePoint) (DoublePoint, bool, error) {
	testDirs := []DoublePoint{
		{X: baseDir.X, Y: baseDir.Y},
		{X: -baseDir.Y, Y: baseDir.X},
		{X: -baseDir.X, Y: -baseDir.Y},
		{X: baseDir.Y, Y: -baseDir.X},
	}
	var bestDir DoublePoint
	bestErr := 0.0
	haveBest := false
	allZero := true
	for _, td := range testDirs {
		it, err := rp.iterateNextStep(toolPos, td, false)
		if err != nil {
			return DoublePoint{}, false, err
		}
		if it.area != 0 {
			allZero = false
		}
		if !it.failed && (!haveBest || it.errorFraction < bestErr) {
			bestDir, bestErr, haveBest = it.newToolDir, it.errorFraction, true
		}
	}
	if haveBest {
		return bestDir, true, nil
	}
	if allZero {
		clearedArea := rp.cleared.cleared()
		near := distancePointToPathsSqrd(clearedArea, toolPos)
		if near.distSqrd < float64(rp.s.toolRadiusScaled*rp.s.toolRadiusScaled) {
			centroid := compute2DPolygonCentroid(clearedArea[near.pathIndex])
			return directionV(toolPos, centroid), true, nil
		}
	}
	return DoublePoint{}, false, nil
}
