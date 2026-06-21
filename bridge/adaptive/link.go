// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

// The lead and link path machinery drives the clipping engine (offsets, cleared-area expansion,
// clear-path tests) so it is built only with cgo; the pure geometry it leans on lives in the
// untagged linkgeom.go.

package adaptive

import (
	"math"
	"time"

	"oblikovati.org/cam/bridge/clipper"
)

// leadStepSlopeAlfa is the per-iteration rotation applied while searching for a clear lead path;
// leadAdaptFactor is how strongly the lead path bends back toward the beacon each step.
const (
	leadStepSlopeAlfa = math.Pi / 64
	leadAdaptFactor   = 0.4
)

// makeLeadPath grows a short arcing entry (leadIn) or exit (leadOut) path from startPoint heading
// startDir, bending toward beaconPoint, that stays clear of uncut stock: it steps along, rotating
// away when a step would over-engage, and finishes once it has reached and travelled a little
// inside the cleared region. For lead-out it expands a working copy of the cleared area as it goes.
// Returns the path and whether it succeeded. Exact port of MakeLeadPath.
func (rp *regionProcessor) makeLeadPath(leadIn bool, startPoint clipper.IntPoint, startDir DoublePoint, beaconPoint clipper.IntPoint, clearedAreaOriginal *clearedArea, toolBoundPaths clipper.Paths) (clipper.Path, bool, error) {
	output := clipper.Path{startPoint}
	stepSize := math.Min(minStepClipper*8, 0.2*rp.s.stepOverScaled+1)

	// working copy of the cleared area, expanded as a lead-out path progresses
	working := newClearedArea(rp.s.toolRadiusScaled)
	working.setClearedPaths(clearedAreaOriginal.cleared())

	// acceptable tool end locations: the cleared area shrunk by tool radius + one step
	cleared, err := clipper.Offset(working.cleared(), clipper.Round, clipper.ClosedPolygon, -(float64(rp.s.toolRadiusScaled) + stepSize), 0, 0)
	if err != nil {
		return nil, false, err
	}
	if len(cleared) == 0 {
		return nil, false, nil
	}

	// move the beacon to an acceptable location if it isn't inside the shrunk area
	if getPathNestingLevel(clipper.Path{beaconPoint}, cleared)%2 == 0 {
		beaconPoint = distancePointToPathsSqrd(cleared, beaconPoint).point
	}

	currentPoint := startPoint
	distanceToBeacon := math.Sqrt(distanceSqrd(startPoint, beaconPoint))
	minExitLength := math.Min(float64(rp.s.toolRadiusScaled)/5, math.Min(rp.s.stepOverScaled, distanceToBeacon/2))
	maxLength := math.Max(distanceToBeacon*2, stepSize*10)
	clearedStartLen := 0.0
	haveClearedStart := false
	nextDir := startDir
	nextPoint := clipper.IntPoint{X: currentPoint.X + int64(nextDir.X*stepSize), Y: currentPoint.Y + int64(nextDir.Y*stepSize)}
	checkPath := clipper.Path{currentPoint}
	pathLen := 0.0

	for i := 0; i < 10000; i++ {
		allowed, err := rp.isAllowedToCutThrough(currentPoint, nextPoint, working, toolBoundPaths, 1.5, false)
		if err != nil {
			return nil, false, err
		}
		if allowed {
			if !leadIn {
				// lead-out: fold the new segment into the working cleared area and recompute the shrunk region
				checkPath = append(checkPath, nextPoint)
				if err := working.expandCleared(checkPath); err != nil {
					return nil, false, err
				}
				checkPath = clipper.Path{nextPoint}
				cleared, err = clipper.Offset(working.cleared(), clipper.Round, clipper.ClosedPolygon, -(float64(rp.s.toolRadiusScaled) + stepSize), 0, 0)
				if err != nil {
					return nil, false, err
				}
			}
			output = append(output, nextPoint)
			currentPoint = nextPoint
			pathLen += stepSize
			targetDir := directionV(currentPoint, beaconPoint)
			nextDir = DoublePoint{X: nextDir.X + leadAdaptFactor*targetDir.X, Y: nextDir.Y + leadAdaptFactor*targetDir.Y}
			normalizeV(&nextDir)

			if getPathNestingLevel(clipper.Path{currentPoint}, cleared)%2 == 1 {
				if !haveClearedStart {
					clearedStartLen = pathLen
					haveClearedStart = true
				}
				if pathLen > minExitLength && pathLen-clearedStartLen > minStepClipper {
					return output, true, nil // reached and travelled a little inside the cleared region
				}
			} else {
				haveClearedStart = false
			}

			if pathLen > maxLength {
				if getPathNestingLevel(clipper.Path{currentPoint}, working.cleared())%2 == 1 {
					return output, true, nil
				}
				rp.output.LeadPathFailed = true
				return nil, false, nil // overtravel without reaching the cleared area
			}
		} else {
			rot := -leadStepSlopeAlfa
			if !leadIn {
				rot = leadStepSlopeAlfa
			}
			nextDir = rotateVec(nextDir, rot)
		}
		nextPoint = clipper.IntPoint{X: currentPoint.X + int64(nextDir.X*stepSize), Y: currentPoint.Y + int64(nextDir.Y*stepSize)}
	}
	return nil, false, nil
}

// pointPair is one start→end candidate segment on the link-resolution stack.
type pointPair struct{ first, second clipper.IntPoint }

// resolveLinkPath finds a keep-tool-down path from startPoint to endPoint that stays over already
// cleared stock: it recursively bisects any segment that isn't clear, pushing the half that heads
// toward more-cleared material first, and gives up if the path self-intersects, grows past
// keepToolDownDistRatio × the direct distance, or a search budget is exhausted. Returns the
// connected link path and whether one was found. Exact port of ResolveLinkPath.
func (rp *regionProcessor) resolveLinkPath(startPoint, endPoint clipper.IntPoint, clearedArea *clearedArea) (clipper.Path, bool, error) {
	queue := []pointPair{{startPoint, endPoint}}
	var linkPaths clipper.Paths
	totalLength := 0.0
	directDistance := math.Sqrt(distanceSqrd(startPoint, endPoint))

	scanStep := math.Min(math.Max(2*minStepClipper, float64(rp.s.scaleFactor)*0.01), float64(rp.s.scaleFactor)*0.1)
	const limit = 10000

	clearance := rp.s.stepOverScaled
	offClearance := 2 * rp.s.stepOverScaled
	if offClearance > directDistance/2 {
		offClearance = directDistance / 2
		clearance = 0
	}

	cnt := 0
	// match the upstream wall-clock budget for resolving a single keep-tool-down link
	deadline := time.Now().Add(time.Duration(math.Max(rp.s.cfg.KeepToolDownDistRatio, 3.0) * float64(time.Second) / 6))

	for len(queue) > 0 {
		if time.Now().After(deadline) {
			return nil, false, nil
		}
		cnt++
		if cnt > limit {
			return nil, false, nil
		}
		pp := queue[len(queue)-1]
		queue = queue[:len(queue)-1]

		if linkSelfIntersects(linkPaths, pp) {
			return nil, false, nil
		}

		direction := directionV(pp.first, pp.second)
		checkPath := linkCheckEndpoints(pp, startPoint, endPoint, direction, offClearance)

		clear, err := rp.isClearPath(checkPath, clearedArea, clearance)
		if err != nil {
			return nil, false, err
		}
		if clear {
			totalLength += math.Sqrt(distanceSqrd(pp.first, pp.second))
			if totalLength > rp.s.cfg.KeepToolDownDistRatio*directDistance {
				return nil, false, nil
			}
			linkPaths = append(linkPaths, clipper.Path{pp.first, pp.second})
			continue
		}
		if math.Sqrt(distanceSqrd(pp.first, pp.second)) < 4 {
			return nil, false, nil // segment too short but still not clear
		}
		ok, err := rp.bisectLink(&queue, clearedArea, pp, direction, scanStep, clearance, directDistance)
		if err != nil {
			return nil, false, err
		}
		if !ok {
			return nil, false, nil // couldn't find a keep-tool-down detour
		}
	}
	if len(linkPaths) == 0 {
		return nil, false, nil
	}
	connected := connectPaths(linkPaths)
	return connected[0], true, nil
}

// linkSelfIntersects reports whether segment pp crosses any already-accepted link segment (sharing
// an endpoint doesn't count). Exact port of the self-intersection guard in ResolveLinkPath.
func linkSelfIntersects(linkPaths clipper.Paths, pp pointPair) bool {
	for i := range linkPaths {
		lf := linkPaths[i][0]
		lb := linkPaths[i][len(linkPaths[i])-1]
		if lf == pp.first || lb == pp.first || lf == pp.second || lb == pp.second {
			continue
		}
		if _, hit := intersectionPoint(lf, lb, pp.first, pp.second); hit {
			return true
		}
	}
	return false
}

// linkCheckEndpoints builds the two-point probe for a candidate link segment, pulling the start/end
// in by offClearance (only at the true path ends) so the clear-path test allows the tool to sit a
// little outside the cleared region at the lead-in/out. Exact port of the checkPath construction.
func linkCheckEndpoints(pp pointPair, startPoint, endPoint clipper.IntPoint, direction DoublePoint, offClearance float64) clipper.Path {
	var checkPath clipper.Path
	if pp.first == startPoint {
		checkPath = append(checkPath, clipper.IntPoint{X: pp.first.X + int64(offClearance*direction.X), Y: pp.first.Y + int64(offClearance*direction.Y)})
	} else {
		checkPath = append(checkPath, pp.first)
	}
	if pp.second == endPoint {
		checkPath = append(checkPath, clipper.IntPoint{X: pp.second.X - int64(offClearance*direction.X), Y: pp.second.Y - int64(offClearance*direction.Y)})
	} else {
		checkPath = append(checkPath, pp.second)
	}
	return checkPath
}

// bisectLink splits an unclear segment at its midpoint, scanning perpendicular to it for the
// nearest clear waypoint (preferring the side closer to cleared stock), and pushes the two
// resulting sub-segments onto the queue. Returns false when no clear detour exists within the
// keep-tool-down budget. Exact port of the bisection block in ResolveLinkPath.
func (rp *regionProcessor) bisectLink(queue *[]pointPair, clearedArea *clearedArea, pp pointPair, direction DoublePoint, scanStep, clearance, directDistance float64) (bool, error) {
	pDir := DoublePoint{X: -direction.Y, Y: direction.X}
	midPoint := clipper.IntPoint{X: int64(0.5 * float64(pp.first.X+pp.second.X)), Y: int64(0.5 * float64(pp.first.Y+pp.second.Y))}
	for i := int64(1); ; i++ {
		offset := float64(i) * scanStep
		checkPoint1 := clipper.IntPoint{X: midPoint.X + int64(offset*pDir.X), Y: midPoint.Y + int64(offset*pDir.Y)}
		checkPoint2 := clipper.IntPoint{X: midPoint.X - int64(offset*pDir.X), Y: midPoint.Y - int64(offset*pDir.Y)}
		// test the more promising side first (the one farther from the cleared-area boundary)
		if distancePointToPathsSqrd(clearedArea.cleared(), checkPoint1).distSqrd < distancePointToPathsSqrd(clearedArea.cleared(), checkPoint2).distSqrd {
			checkPoint1, checkPoint2 = checkPoint2, checkPoint1
		}
		c1, err := rp.isClearPath(clipper.Path{checkPoint1}, clearedArea, clearance+1)
		if err != nil {
			return false, err
		}
		if c1 {
			*queue = append(*queue, pointPair{pp.first, checkPoint1}, pointPair{checkPoint1, pp.second})
			return true, nil
		}
		c2, err := rp.isClearPath(clipper.Path{checkPoint2}, clearedArea, clearance+1)
		if err != nil {
			return false, err
		}
		if c2 {
			*queue = append(*queue, pointPair{pp.first, checkPoint2}, pointPair{checkPoint2, pp.second})
			return true, nil
		}
		if offset > rp.s.cfg.KeepToolDownDistRatio*directDistance {
			return false, nil
		}
	}
}
