// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

// FindLinkPath plans the move from the previous cut to the next engage point (link + lead-in);
// AppendToolPath emits a finished pass (its link, the cut, and a planned lead-out) into the output.
// Both drive the clipping engine through the lead/link solvers, so they are cgo-only.

package adaptive

import (
	"math"

	"oblikovati.org/cam/bridge/clipper"
)

// scaleToMM converts a scaled-plane integer path into millimetre DoublePoints for the output.
func (rp *regionProcessor) scaleToMM(path clipper.Path) DPath {
	out := make(DPath, len(path))
	sf := float64(rp.s.scaleFactor)
	for i, p := range path {
		out[i] = DoublePoint{X: float64(p.X) / sf, Y: float64(p.Y) / sf}
	}
	return out
}

// findLinkPath plans how the tool reaches the next pass's engage point (pathStart, heading pathDir):
// it builds a lead-in by running makeLeadPath as a reverse lead-out toward a beacon placed to the
// side of the path, then — if there is a previous position — a linking move to that lead-in (a
// keep-tool-down route when one exists, otherwise a direct clear-height move, possibly cut if
// short and safe). Returns the link+lead-in as scaled-out TPaths and whether it succeeded. Exact
// port of FindLinkPath. A nil prevPoint means there is no prior cut (first engagement).
func (rp *regionProcessor) findLinkPath(prevPoint *clipper.IntPoint, pathStart clipper.IntPoint, pathDir DoublePoint, cleared *clearedArea, toolBoundPaths clipper.Paths) ([]TPath, bool, error) {
	var result []TPath
	endPoint := pathStart

	linkDistance := rp.s.stepOverScaled
	if prevPoint != nil {
		linkDistance = math.Sqrt(distanceSqrd(*prevPoint, endPoint))
	}
	if linkDistance < numericTolerance {
		return result, true, nil // very short link: no special linking required
	}

	beaconOffset := math.Max(math.Min(rp.s.stepOverScaled, linkDistance/2)*1.5, 8*minStepClipper)

	// plan the lead-in as a reverse-direction lead-out, leaving the path toward a side beacon
	near := distancePointToPathsSqrd(toolBoundPaths, endPoint)
	revEndDir := DoublePoint{X: -pathDir.X, Y: -pathDir.Y}
	endBoundaryDir := getPathDirectionV(toolBoundPaths[near.pathIndex], near.segmentIndex)
	if near.distSqrd > beaconOffset {
		endBoundaryDir = pathDir // boundary far away → use the beacon to leave the path
	}
	endBeaconDir := DoublePoint{X: revEndDir.X - endBoundaryDir.Y, Y: revEndDir.Y + endBoundaryDir.X}
	normalizeV(&endBeaconDir)
	endBeacon := clipper.IntPoint{X: endPoint.X + int64(beaconOffset*endBeaconDir.X), Y: endPoint.Y + int64(beaconOffset*endBeaconDir.Y)}

	leadInPath, ok, err := rp.makeLeadPath(true, endPoint, revEndDir, endBeacon, cleared, toolBoundPaths)
	if err != nil {
		return nil, false, err
	}
	clipper.ReversePath(leadInPath)
	if !ok {
		return nil, false, nil
	}

	var linkPath clipper.Path
	linkType := MotionCutting
	if prevPoint != nil {
		linkType, linkPath, err = rp.buildLinkToLeadIn(*prevPoint, &leadInPath, cleared, toolBoundPaths)
		if err != nil {
			return nil, false, err
		}
	}

	linkPaths := clipper.Paths{linkPath, leadInPath}
	if linkType == MotionLinkClear {
		linkPaths = smoothPaths(linkPaths, 0.1*rp.s.stepOverScaled, 1, 4)
	}
	linkPath, leadInPath = linkPaths[0], linkPaths[1]

	if prevPoint != nil {
		result = append(result, TPath{Motion: linkType, Pts: rp.scaleToMM(linkPath)})
	}
	result = append(result, TPath{Motion: MotionCutting, Pts: rp.scaleToMM(leadInPath)})
	return result, true, nil
}

// buildLinkToLeadIn builds the move from prevPoint to the start of leadInPath. When a keep-tool-down
// route exists it is a clear link (and its tail is folded into the lead-in by extendLeadInAlongLink);
// otherwise it is a direct move at clear height, marked as a cut when it is short and safe enough to
// cut directly. Exact port of the prevPoint block of FindLinkPath.
func (rp *regionProcessor) buildLinkToLeadIn(prevPoint clipper.IntPoint, leadInPath *clipper.Path, cleared *clearedArea, toolBoundPaths clipper.Paths) (MotionType, clipper.Path, error) {
	linkPath, ok, err := rp.resolveLinkPath(prevPoint, (*leadInPath)[0], cleared)
	if err != nil {
		return MotionCutting, nil, err
	}
	if ok {
		if err := rp.extendLeadInAlongLink(&linkPath, leadInPath, cleared); err != nil {
			return MotionCutting, nil, err
		}
		return MotionLinkClear, linkPath, nil
	}

	linkType := MotionLinkNotClear
	front := (*leadInPath)[0]
	dist := math.Sqrt(distanceSqrd(prevPoint, front))
	if dist < 2*rp.s.stepOverScaled {
		dx := float64(front.X-prevPoint.X) / dist
		dy := float64(front.Y-prevPoint.Y) / dist
		p1 := clipper.IntPoint{X: prevPoint.X + int64(dx), Y: prevPoint.Y + int64(dy)}
		p2 := clipper.IntPoint{X: front.X - int64(dx), Y: front.Y - int64(dy)}
		allowed, err := rp.isAllowedToCutThrough(p1, p2, cleared, toolBoundPaths, 1.5, false)
		if err != nil {
			return MotionCutting, nil, err
		}
		if allowed {
			linkType = MotionCutting
		}
	}
	return linkType, clipper.Path{prevPoint, front}, nil
}

// extendLeadInAlongLink walks back along the clear link, moving up to stepOver/2 of its tail into
// the front of the lead-in so the engaged cut starts a little earlier along the already-clear route,
// stopping early when a moved span would no longer be clear. Exact port of the lead-in extension
// while-loop. Both paths are modified in place.
func (rp *regionProcessor) extendLeadInAlongLink(linkPath, leadInPath *clipper.Path, cleared *clearedArea) error {
	remaining := rp.s.stepOverScaled / 2
	for len(*linkPath) >= 2 && remaining > numericTolerance {
		lp := *linkPath
		p1 := lp[len(lp)-2]
		p2 := lp[len(lp)-1]
		l := math.Sqrt(distanceSqrd(p1, p2))
		if l >= remaining {
			split := clipper.IntPoint{
				X: p1.X + int64(float64(p2.X-p1.X)*(l-remaining)/l),
				Y: p1.Y + int64(float64(p2.Y-p1.Y)*(l-remaining)/l),
			}
			*linkPath = append(lp[:len(lp)-1], split)
			*leadInPath = append(clipper.Path{split}, *leadInPath...)
			remaining = 0
			clear, err := rp.isClearPath(clipper.Path{p2, split}, cleared, 0)
			if err != nil {
				return err
			}
			if !clear {
				remaining = rp.s.stepOverScaled / 2
			}
		} else {
			*linkPath = lp[:len(lp)-1]
			*leadInPath = append(clipper.Path{p1}, *leadInPath...)
			remaining -= l
			if remaining < numericTolerance {
				clear, err := rp.isClearPath(clipper.Path{p2, p1}, cleared, 0)
				if err != nil {
					return err
				}
				if !clear {
					remaining = rp.s.stepOverScaled / 2
				}
			}
		}
	}
	return nil
}

// appendToolPath emits a completed pass into the output: first its link/lead-in TPaths, then the
// engaged cut, then a planned lead-out that leaves the just-cut path toward a side beacon (smoothed,
// and folded into the cleared area). Returns the tool position/direction at the end of the lead-out
// (so the next link starts there) and whether a lead-out was produced. Exact port of AppendToolPath.
func (rp *regionProcessor) appendToolPath(passToolPath clipper.Path, linkPath []TPath, cleared *clearedArea, toolBoundPaths clipper.Paths) (clipper.IntPoint, DoublePoint, bool, error) {
	rp.output.AdaptivePaths = append(rp.output.AdaptivePaths, linkPath...)

	cutPts := rp.scaleToMM(passToolPath)
	if len(cutPts) == 0 {
		return clipper.IntPoint{}, DoublePoint{}, false, nil
	}
	rp.output.AdaptivePaths = append(rp.output.AdaptivePaths, TPath{Motion: MotionCutting, Pts: cutPts})

	if len(passToolPath) < 2 {
		return clipper.IntPoint{}, DoublePoint{}, false, nil
	}

	prevPoint := passToolPath[len(passToolPath)-1]
	prevDir := getPathDirectionV(passToolPath, len(passToolPath)-1)
	near := distancePointToPathsSqrd(toolBoundPaths, prevPoint)
	boundaryDir := getPathDirectionV(toolBoundPaths[near.pathIndex], near.segmentIndex)
	beaconOffset := math.Max(math.Min(rp.s.stepOverScaled, pathLength(passToolPath)/2)*1.5, 8*minStepClipper)
	if near.distSqrd > beaconOffset {
		boundaryDir = prevDir // boundary far away → use the beacon to leave the path
	}
	beaconDir := DoublePoint{X: prevDir.X - boundaryDir.Y, Y: prevDir.Y + boundaryDir.X}
	normalizeV(&beaconDir)
	beacon := clipper.IntPoint{X: prevPoint.X + int64(beaconOffset*beaconDir.X), Y: prevPoint.Y + int64(beaconOffset*beaconDir.Y)}

	leadOutPath, ok, err := rp.makeLeadPath(false, prevPoint, prevDir, beacon, cleared, toolBoundPaths)
	if err != nil {
		return clipper.IntPoint{}, DoublePoint{}, false, err
	}
	if !ok || len(leadOutPath) < 1 {
		return clipper.IntPoint{}, DoublePoint{}, false, nil
	}

	leadOutPath = smoothPaths(clipper.Paths{leadOutPath}, 0.1*rp.s.stepOverScaled, 1, 4)[0]
	rp.output.AdaptivePaths = append(rp.output.AdaptivePaths, TPath{Motion: MotionCutting, Pts: rp.scaleToMM(leadOutPath)})
	if err := cleared.expandCleared(leadOutPath); err != nil {
		return clipper.IntPoint{}, DoublePoint{}, false, err
	}

	p2 := leadOutPath[len(leadOutPath)-1]
	p1 := prevPoint
	if len(leadOutPath) >= 2 {
		p1 = leadOutPath[len(leadOutPath)-2]
	}
	return p2, directionV(p1, p2), true, nil
}
