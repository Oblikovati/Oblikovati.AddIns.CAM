// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

// Execute is the public entry point: it prepares the input geometry, splits it into connected
// regions, and clears each one. It drives the clipping engine throughout (cgo only). Profiling op
// types and the !forceInsideOut stock-overshoot path are not yet supported (see the fidelity notes);
// the supported set — inside/outside clearing with forceInsideOut — covers adaptive pocketing.

package adaptive

import (
	"fmt"

	"oblikovati.org/cam/bridge/clipper"
)

// boundShrinkGuard shrinks the region boundary by a few units so rounding in the offset chain can
// only ever shrink it — important when filtering out regions already covered by the cleared area.
const boundShrinkGuard = 3

// Execute clears the regions bounded by paths (all in millimetres), leaving stockToLeave on the
// walls and honouring any already-cleared area. It returns one Output per connected region (helix
// centre, start point, and the adaptive/link toolpaths). stockPaths bounds the raw stock (used to
// skip finishing cuts in thin air); clearedPaths is material already removed. Exact port of
// Adaptive2d::Execute for the clearing op types.
func Execute(cfg Config, stockPaths, paths, clearedPaths []DPath) ([]Output, error) {
	if cfg.OpType == ProfilingInside || cfg.OpType == ProfilingOutside {
		return nil, fmt.Errorf("adaptive.Execute: profiling op type %d not supported (clearing only)", cfg.OpType)
	}
	s := newSolver(cfg)
	if err := s.buildToolGeometry(); err != nil {
		return nil, err
	}

	inputPaths, err := s.prepareInputPaths(paths)
	if err != nil {
		return nil, err
	}
	stockInputPaths, err := clipper.Simplify(s.scalePaths(stockPaths), clipper.EvenOdd)
	if err != nil {
		return nil, err
	}
	initialCleared, err := clipper.Simplify(s.scalePaths(clearedPaths), clipper.EvenOdd)
	if err != nil {
		return nil, err
	}

	if cfg.OpType == ClearingOutside {
		inputPaths = append(inputPaths, stockInputPaths...)
	}
	fixOrientation(inputPaths)

	// every input path needs a finishing pass (upstream tags them Z=1); the only paths that would be
	// exempt come from the profiling / !forceInsideOut branches, which are not ported here.
	toolBounds, err := perCurveOffset(inputPaths, inputPaths, -float64(s.toolRadiusScaled+s.finishPassOffsetScaled))
	if err != nil {
		return nil, err
	}
	return s.clearRegions(toolBounds, stockInputPaths, initialCleared)
}

// clearRegions walks the connected components of the tool bounds (each exterior ring with its
// direct-child holes), builds the boundary and finishing paths for each, skips any already covered
// by the cleared area, and clears it. Exact port of the region loop (steps 7–10).
func (s *solver) clearRegions(toolBounds, stockInputPaths, initialCleared clipper.Paths) ([]Output, error) {
	var outputs []Output
	for _, current := range toolBounds {
		nesting := getPathNestingLevel(current, toolBounds)
		if nesting%2 == 0 {
			continue // a hole, not an exterior boundary
		}
		currentTBP := directChildRegion(current, toolBounds, nesting)
		finishingPass, err := perCurveOffset(currentTBP, toolBounds, float64(s.finishPassOffsetScaled))
		if err != nil {
			return nil, err
		}
		boundPath, err := clipper.Offset(currentTBP, clipper.Round, clipper.ClosedPolygon, float64(s.toolRadiusScaled-boundShrinkGuard), 0, 0)
		if err != nil {
			return nil, err
		}
		cleared, err := alreadyCleared(boundPath, initialCleared)
		if err != nil {
			return nil, err
		}
		if cleared {
			continue
		}
		out := &Output{}
		rp, err := newRegionProcessor(s, boundPath, currentTBP, initialCleared, out)
		if err != nil {
			return nil, err
		}
		if err := rp.process(stockInputPaths, finishingPass); err != nil {
			return nil, err
		}
		outputs = append(outputs, *out)
	}
	return outputs, nil
}

// directChildRegion gathers an exterior ring together with the holes nested one level directly
// inside it, the (boundary + holes) set that defines one connected region. Exact port.
func directChildRegion(current clipper.Path, toolBounds clipper.Paths, nesting int) clipper.Paths {
	region := clipper.Paths{current}
	for _, other := range toolBounds {
		if len(other) == 0 {
			continue
		}
		if clipper.PointInPolygon(other[0], current) != 0 && getPathNestingLevel(other, toolBounds) == nesting+1 {
			region = append(region, other)
		}
	}
	return region
}

// alreadyCleared reports whether the boundary lies entirely within the initial cleared area (so the
// region needs no work). Exact port of the boundsToClear emptiness check.
func alreadyCleared(boundPath, initialCleared clipper.Paths) (bool, error) {
	remaining, err := clipper.Subtract(boundPath, initialCleared)
	if err != nil {
		return false, err
	}
	return len(remaining) == 0, nil
}

// prepareInputPaths scales the input contours into the integer plane, cleans, deduplicates and
// reconnects them, simplifies, and applies the stock-to-leave offset. Exact port of the input-path
// conversion block.
func (s *solver) prepareInputPaths(paths []DPath) (clipper.Paths, error) {
	var converted clipper.Paths
	for _, dp := range paths {
		converted = append(converted, cleanPath(s.scalePath(dp), cleanPathTolerance))
	}
	connected := connectPaths(deduplicatePaths(converted))
	simplified, err := clipper.Simplify(connected, clipper.EvenOdd)
	if err != nil {
		return nil, err
	}
	return s.applyStockToLeave(simplified)
}

// applyStockToLeave offsets the input paths inward (or outward for outside ops) by the stock-to-leave
// amount; with no stock to leave it runs the upstream −1/+1 offset-and-filter glitch fix. Exact port
// of ApplyStockToLeave.
func (s *solver) applyStockToLeave(inputPaths clipper.Paths) (clipper.Paths, error) {
	if s.cfg.StockToLeave > numericTolerance {
		delta := -s.cfg.StockToLeave * float64(s.scaleFactor)
		if s.cfg.OpType == ClearingOutside || s.cfg.OpType == ProfilingOutside {
			delta = -delta
		}
		return clipper.Offset(inputPaths, clipper.Round, clipper.ClosedPolygon, delta, 0, 0)
	}
	shrunk, err := clipper.Offset(inputPaths, clipper.Round, clipper.ClosedPolygon, -1, 0, 0)
	if err != nil {
		return nil, err
	}
	filterCloseValues(shrunk)
	grown, err := clipper.Offset(shrunk, clipper.Round, clipper.ClosedPolygon, 1, 0, 0)
	if err != nil {
		return nil, err
	}
	filterCloseValues(grown)
	return grown, nil
}

// fixOrientation orients each path so exterior boundaries (odd nesting) wind positive and holes wind
// negative. Exact port of the orientation-fix loop.
func fixOrientation(inputPaths clipper.Paths) {
	for i := range inputPaths {
		odd := getPathNestingLevel(inputPaths[i], inputPaths)%2 == 1
		if odd != clipper.Orientation(inputPaths[i]) {
			clipper.ReversePath(inputPaths[i])
		}
	}
}

// perCurveOffset offsets each path individually by delta scaled by its nesting direction (+1 for
// exterior, −1 for holes), restoring the source orientation on the result. Offsetting per curve
// (rather than the whole set) matches the upstream, which does it that way to preserve per-path
// data the set offset would drop. reference supplies the nesting context.
func perCurveOffset(paths, reference clipper.Paths, delta float64) (clipper.Paths, error) {
	var out clipper.Paths
	for _, path := range paths {
		orientation := clipper.Orientation(path)
		direction := -1.0
		if getPathNestingLevel(path, reference)%2 == 1 {
			direction = 1.0
		}
		res, err := clipper.Offset(clipper.Paths{path}, clipper.Round, clipper.ClosedPolygon, delta*direction, 0, 0)
		if err != nil {
			return nil, err
		}
		for _, p := range res {
			if clipper.Orientation(p) != orientation {
				clipper.ReversePath(p)
			}
			out = append(out, p)
		}
	}
	return out, nil
}

// scalePath converts a millimetre contour into the integer working plane.
func (s *solver) scalePath(dp DPath) clipper.Path {
	out := make(clipper.Path, len(dp))
	sf := float64(s.scaleFactor)
	for i, p := range dp {
		out[i] = clipper.IntPoint{X: int64(p.X * sf), Y: int64(p.Y * sf)}
	}
	return out
}

// scalePaths scales a set of millimetre contours.
func (s *solver) scalePaths(dps []DPath) clipper.Paths {
	out := make(clipper.Paths, len(dps))
	for i, dp := range dps {
		out[i] = s.scalePath(dp)
	}
	return out
}
