// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

// The finishing pass walks the precomputed finishing wall paths after the bulk has been cleared,
// linking to and cutting each one. process() is the whole ProcessPolyNode: clear the region, then
// (optionally) finish it. cgo-only, like the rest of the clearing machinery.

package adaptive

import (
	"math"

	"oblikovati.org/cam/bridge/clipper"
)

// finishWallOffset is how far the finishing tool-bound is grown/shrunk off each finishing path so a
// finishing lead-in has somewhere to sit.
const finishWallOffset = 3.0

// process clears one region and then runs the finishing wall pass when finishing is enabled. It is
// the full ProcessPolyNode. Returns without finishing if the region could not be entered.
func (rp *regionProcessor) process(stockInputPaths, finishingPaths clipper.Paths) error {
	if err := rp.clearRegion(); err != nil {
		return err
	}
	if rp.output.StartPointNotFound {
		return nil
	}
	if rp.s.cfg.FinishingProfile {
		if err := rp.finishingPass(stockInputPaths, finishingPaths); err != nil {
			return err
		}
	}
	return rp.finalizeClearedArea()
}

// finishingPass cuts the thin finishing allowance along the walls: it retargets the tool bound to
// the finishing paths, then repeatedly takes the finishing path closest to the tool, skips any that
// lie outside the stock, links to it and cuts it, folding it into the cleared area. Finally it sets
// the region's return-motion type from whether the tool can rapid back to the entry over cleared
// stock. Exact port of the finishing-pass block of ProcessPolyNode.
func (rp *regionProcessor) finishingPass(stockInputPaths, finishingPaths clipper.Paths) error {
	rp.toolBoundPaths = finishingToolBound(finishingPaths)

	remaining := finishingPaths
	for len(remaining) > 0 {
		var finShifted clipper.Path
		var ok bool
		finShifted, remaining, ok = popPathWithClosestPoint(remaining, rp.toolPos, rp.s.stepOverScaled)
		if !ok {
			break
		}
		if len(finShifted) == 0 || allPointsOutsideStock(finShifted, stockInputPaths) {
			continue
		}
		if err := rp.cutFinishingPath(finShifted); err != nil {
			return err
		}
	}

	returnPath := clipper.Path{rp.toolPos, rp.entryPoint}
	clear, err := rp.isClearPath(returnPath, rp.cleared, 0)
	if err != nil {
		return err
	}
	if clear {
		rp.output.ReturnMotion = MotionLinkClear
	} else {
		rp.output.ReturnMotion = MotionLinkNotClear
	}
	return nil
}

// finishingToolBound rebuilds the tool-bound paths around the finishing paths: each finishing path
// is offset outward (solid) or inward (hole) by a small amount and reoriented to match, so the
// finishing lead-ins resolve against a bound that hugs the walls. Exact port of the tbpModified
// block.
func finishingToolBound(finishingPaths clipper.Paths) clipper.Paths {
	var tbp clipper.Paths
	for _, fp := range finishingPaths {
		delta := -finishWallOffset
		if getPathNestingLevel(fp, finishingPaths)%2 == 1 {
			delta = finishWallOffset
		}
		out, err := clipper.Offset(clipper.Paths{fp}, clipper.Round, clipper.ClosedPolygon, delta, 0, 0)
		if err != nil || len(out) == 0 {
			continue
		}
		orientation := clipper.Orientation(fp)
		for _, p := range out {
			if clipper.Orientation(p) != orientation {
				clipper.ReversePath(p)
			}
			tbp = append(tbp, p)
		}
	}
	return tbp
}

// allPointsOutsideStock reports whether a finishing path lies entirely outside the stock (so there
// is no material to finish): it checks each vertex and each segment midpoint. Exact port.
func allPointsOutsideStock(path clipper.Path, stockInputPaths clipper.Paths) bool {
	prev := path[0]
	for _, pt := range path {
		mid := clipper.IntPoint{X: (prev.X + pt.X) / 2, Y: (prev.Y + pt.Y) / 2}
		if isPointWithinCutRegion(stockInputPaths, mid) || isPointWithinCutRegion(stockInputPaths, pt) {
			return false
		}
		prev = pt
	}
	return true
}

// cutFinishingPath links to a single finishing path and cuts it: it closes and cleans the path,
// resolves a lead-in, appends it, and folds it (and its cutting link) into the cleared area. Exact
// port of the per-finishing-path body.
func (rp *regionProcessor) cutFinishingPath(finShifted clipper.Path) error {
	finShifted = append(finShifted, finShifted[0]) // close
	finCleaned := cleanPath(finShifted, cleanPathTolerance)
	if math.Sqrt(distanceSqrd(finCleaned[0], finCleaned[len(finCleaned)-1])) < cleanPathTolerance {
		finCleaned = finCleaned[:len(finCleaned)-1]
	}
	finCleaned = append(finCleaned, finCleaned[0]) // ensure closed without ruining the final direction

	link, ok, err := rp.findLinkPath(&rp.toolPos, finCleaned[0], getPathDirectionV(finCleaned, 1), rp.cleared, rp.toolBoundPaths)
	if err != nil {
		return err
	}
	if !ok {
		rp.output.FinishingLeadInFailed = true
		return nil
	}

	pos, dir, gotPos, err := rp.appendToolPath(finCleaned, link, rp.cleared, rp.toolBoundPaths)
	if err != nil {
		return err
	}
	if gotPos {
		rp.toolPos, rp.toolDir = pos, dir
	} else {
		rp.toolPos = finCleaned[len(finCleaned)-1]
		rp.toolDir = getPathDirectionV(finCleaned, len(finCleaned)-1)
	}

	if err := rp.cleared.expandCleared(finCleaned); err != nil {
		return err
	}
	for _, lp := range link {
		if lp.Motion == MotionCutting {
			if err := rp.cleared.expandCleared(rp.scaleToClipperPath(lp.Pts)); err != nil {
				return err
			}
		}
	}
	return nil
}
