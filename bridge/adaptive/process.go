// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

// clearRegion is the heart of ProcessPolyNode: it bores in, then repeatedly steps the engagement
// loop pass by pass — appending each pass and linking to the next engage point — until the region
// is cleared. It drives the clipping engine throughout, so it is cgo-only. The finishing pass is
// ported separately (see finishing.go).

package adaptive

import (
	"math"

	"oblikovati.org/cam/bridge/clipper"
)

const (
	directionSmoothingBuflen = 3       // gyro length used to smooth the tool direction
	angleHistoryPoints       = 3       // deflection-angle samples kept for prediction
	passesLimit              = 1 << 30 // upstream uses an effectively-unbounded debug limit
	pointsPerPassLimit       = 1 << 30 // ditto, per pass
	badEngageLimit           = 10000   // give up after this many passes that clear no new area
)

// newRegionProcessor prepares a processor to clear one connected region: it cleans and simplifies
// the boundary and tool-bound paths, builds the slightly-shrunk tool bound (tbpMinus) the engage
// search clips against, and seeds the cleared area with whatever was already cleared. Exact port of
// the ProcessPolyNode preamble.
func newRegionProcessor(s *solver, boundPaths, toolBoundPaths, initialClearedPaths clipper.Paths, output *Output) (*regionProcessor, error) {
	tbp, err := cleanSimplify(toolBoundPaths)
	if err != nil {
		return nil, err
	}
	bp, err := cleanSimplify(boundPaths)
	if err != nil {
		return nil, err
	}
	shrunk, err := clipper.Offset(tbp, clipper.Round, clipper.ClosedPolygon, -2, 0, 0)
	if err != nil {
		return nil, err
	}
	tbpMinus, err := cleanSimplify(shrunk)
	if err != nil {
		return nil, err
	}
	cleared := newClearedArea(s.toolRadiusScaled)
	cleared.setClearedPaths(initialClearedPaths)
	return &regionProcessor{
		s:              s,
		toolBoundPaths: tbp,
		boundPaths:     bp,
		cleared:        cleared,
		tbpMinus:       tbpMinus,
		output:         output,
		angle:          math.Pi,
	}, nil
}

// cleanSimplify applies Clipper's CleanPolygons then SimplifyPolygons, the pair the upstream uses to
// normalise its boundary paths.
func cleanSimplify(paths clipper.Paths) (clipper.Paths, error) {
	return clipper.Simplify(clipper.CleanPolygons(paths, cleanPathTolerance), clipper.EvenOdd)
}

// clearRegion bores into the region (engage point or helix), then runs the pass loop until the
// region is cleared. It fills the output's start point and helix centre. Returns without error when
// the region cannot be entered (the output is flagged). Exact port of ProcessPolyNode up to the
// finishing pass.
func (rp *regionProcessor) clearRegion() error {
	rp.clearedBeforePass = newClearedArea(rp.s.toolRadiusScaled)
	rp.clearedBeforePass.setClearedPaths(rp.cleared.cleared())
	rp.lastExpandToolDir = rp.toolDir

	entered, err := rp.seedEntry()
	if err != nil || !entered {
		return err
	}
	rp.output.ReturnMotion = MotionCutting
	sf := float64(rp.s.scaleFactor)
	rp.output.HelixCenter = DoublePoint{X: float64(rp.entryPoint.X) / sf, Y: float64(rp.entryPoint.Y) / sf}
	return rp.runPasses()
}

// seedEntry establishes the first tool position: it prefers an engage point on the cleared border
// (cheap, no plunge), and falls back to boring a helix when none exists (the usual case at the very
// start of a region, where nothing is cleared yet). Returns whether the region was entered. Exact
// port of the entry block of ProcessPolyNode.
func (rp *regionProcessor) seedEntry() (bool, error) {
	sf := float64(rp.s.scaleFactor)
	engage, err := rp.getEngagePoint(nil, rp.toolBoundPaths, rp.tbpMinus)
	if err != nil {
		return false, err
	}
	if engage != nil {
		rp.toolPos = engage.pos
		rp.toolDir = engage.dir
		rp.linkPath = engage.link
		rp.entryPoint = rp.toolPos
		if len(engage.link) > 0 && len(engage.link[0].Pts) > 0 {
			first := engage.link[0].Pts[0]
			rp.entryPoint = clipper.IntPoint{X: int64(first.X * sf), Y: int64(first.Y * sf)}
		}
		rp.output.StartPoint = DoublePoint{X: float64(rp.entryPoint.X) / sf, Y: float64(rp.entryPoint.Y) / sf}
		return true, nil
	}

	res, err := rp.s.findEntryPoint(rp.toolBoundPaths, rp.boundPaths, rp.cleared)
	if err != nil {
		return false, err
	}
	if !res.found {
		rp.output.StartPointNotFound = true
		return false, nil
	}
	rp.entryPoint = res.entryPoint
	rp.toolPos = res.toolPos
	rp.toolDir = res.toolDir
	rp.helixRadiusScaled = res.helixRadiusScaled
	rp.linkPath = nil
	rp.output.StartPoint = DoublePoint{X: float64(rp.toolPos.X) / sf, Y: float64(rp.toolPos.Y) / sf}
	return true, nil
}

// runPasses repeats: cut one pass, and if it cleared new material append it and link to the next
// engage point — until no engagement is left or too many passes clear nothing. Exact port of the
// PASSES loop.
func (rp *regionProcessor) runPasses() error {
	badEngage := 0
	for pass := 0; pass < passesLimit; pass++ {
		passToolPath, err := rp.runPointsLoop()
		if err != nil {
			return err
		}
		cumArea, err := rp.measureNewlyCleared()
		if err != nil {
			return err
		}
		if cumArea >= 1 {
			cleaned := cleanPath(passToolPath, cleanPathTolerance)
			pos, dir, ok, err := rp.appendToolPath(cleaned, rp.linkPath, rp.cleared, rp.toolBoundPaths)
			if err != nil {
				return err
			}
			if ok {
				rp.toolPos, rp.toolDir = pos, dir
			}
			badEngage = 0
		} else {
			badEngage++
		}
		if badEngage > badEngageLimit {
			rp.output.TooManyFailedEngagements = true
			return nil
		}
		rp.clearedBeforePass.setClearedPaths(rp.cleared.cleared())
		cont, err := rp.advanceToNextEngage()
		if err != nil {
			return err
		}
		if !cont {
			return nil
		}
	}
	return nil
}

// runPointsLoop cuts one pass: it folds the link into the cleared area, seeds the gyro and angle
// history, then steps the engagement loop (smoothing the direction through the gyro) building the
// pass toolpath until a step fails, expanding the cleared area as the cut turns. Returns the pass
// toolpath. Exact port of the POINTS loop.
func (rp *regionProcessor) runPointsLoop() (clipper.Path, error) {
	rp.toClearPath = nil
	rp.angleHistory = []float64{0}
	if err := rp.includeLinkInCleared(); err != nil {
		return nil, err
	}
	rp.angle = math.Pi / 4
	rp.gyro = make([]DoublePoint, directionSmoothingBuflen)
	for i := range rp.gyro {
		rp.gyro[i] = rp.toolDir
	}

	var passToolPath clipper.Path
	for pt := 0; pt < pointsPerPassLimit; pt++ {
		rp.toolDir = averageDirection(rp.gyro)
		it, err := rp.iterateNextStep(rp.toolPos, rp.toolDir, true)
		if err != nil {
			return nil, err
		}
		if it.failed {
			break
		}
		if it.hasIterationAngle {
			rp.angleHistory = append(rp.angleHistory, it.iterationAngle)
			if len(rp.angleHistory) > angleHistoryPoints {
				rp.angleHistory = rp.angleHistory[1:]
			}
		}
		if err := rp.maybeExpandOnTurn(it.newToolDir); err != nil {
			return nil, err
		}
		if len(rp.toClearPath) == 0 {
			rp.toClearPath = append(rp.toClearPath, rp.toolPos)
		}
		rp.toClearPath = append(rp.toClearPath, it.newToolPos)
		if len(passToolPath) == 0 {
			passToolPath = append(passToolPath, rp.toolPos)
		}
		passToolPath = append(passToolPath, it.newToolPos)
		rp.toolPos = it.newToolPos
		rp.gyro = append(rp.gyro[1:], it.newToolDir)
	}
	if len(rp.toClearPath) > 0 {
		if err := rp.cleared.expandCleared(rp.toClearPath); err != nil {
			return nil, err
		}
		rp.toClearPath = nil
	}
	return passToolPath, nil
}

// includeLinkInCleared folds the cutting/clear-link parts of the current link into the cleared area
// before a pass starts.
func (rp *regionProcessor) includeLinkInCleared() error {
	for _, lp := range rp.linkPath {
		if lp.Motion == MotionCutting || lp.Motion == MotionLinkClear {
			if err := rp.cleared.expandCleared(rp.scaleToClipperPath(lp.Pts)); err != nil {
				return err
			}
		}
	}
	return nil
}

// maybeExpandOnTurn expands the cleared area by the accumulated cut whenever the tool has turned
// more than 45° from the last expansion (so the cleared-area model keeps up with sharp turns).
func (rp *regionProcessor) maybeExpandOnTurn(newToolDir DoublePoint) error {
	if rp.lastExpandToolDir.X*newToolDir.X+rp.lastExpandToolDir.Y*newToolDir.Y < math.Cos(math.Pi/4) {
		if err := rp.cleared.expandCleared(rp.toClearPath); err != nil {
			return err
		}
		rp.toClearPath = nil
		rp.lastExpandToolDir = rp.toolDir
	}
	return nil
}

// measureNewlyCleared returns the signed area cleared since the last pass (holes subtract), the
// upstream test for "did this pass actually remove material". Exact port.
func (rp *regionProcessor) measureNewlyCleared() (float64, error) {
	newly, err := clipper.Subtract(rp.cleared.cleared(), rp.clearedBeforePass.cleared())
	if err != nil {
		return 0, err
	}
	cum := 0.0
	for _, a := range newly {
		sign := -1.0
		if getPathNestingLevel(a, newly)%2 == 1 {
			sign = 1.0
		}
		cum += sign * math.Abs(clipper.Area(a))
	}
	return cum, nil
}

// advanceToNextEngage finds the next engage point and links to it; if none is left it decides
// whether the region is fully cleared or some unreachable island remains (flagging the latter).
// Returns whether the pass loop should continue. Exact port of the engage/remaining tail.
func (rp *regionProcessor) advanceToNextEngage() (bool, error) {
	prev := rp.toolPos
	engage, err := rp.getEngagePoint(&prev, rp.toolBoundPaths, rp.tbpMinus)
	if err != nil {
		return false, err
	}
	if engage != nil {
		rp.toolPos = engage.pos
		rp.toolDir = engage.dir
		rp.linkPath = engage.link
		rp.lastExpandToolDir = rp.toolDir
		return true, nil
	}
	if rp.unclearedRemains() {
		rp.output.UnclearedAreaRemains = true
	}
	return false, nil
}

// unclearedRemains reports whether any cleared island sits inside the tool bounds yet far from the
// true boundary — material the engage search could not reach. Exact port of the remaining-area scan.
func (rp *regionProcessor) unclearedRemains() bool {
	r2 := float64(4 * rp.s.toolRadiusScaled * rp.s.toolRadiusScaled)
	for _, p := range rp.cleared.cleared() {
		if len(p) == 0 {
			continue
		}
		if isPointWithinCutRegion(rp.toolBoundPaths, p[0]) && distancePointToPathsSqrd(rp.boundPaths, p[0]).distSqrd > r2 {
			return true
		}
	}
	return false
}
