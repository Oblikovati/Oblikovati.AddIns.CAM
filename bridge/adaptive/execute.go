// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

// Execute is the public entry point: it prepares the input geometry, splits it into connected
// regions, and clears each one. It drives the clipping engine throughout (cgo only). It supports the
// clearing op types (inside/outside) with the !forceInsideOut stock-overshoot path (step 5); the
// profiling op types (step 4) are handled in profiling.go.

package adaptive

import "oblikovati.org/cam/bridge/clipper"

// boundShrinkGuard shrinks the region boundary by a few units so rounding in the offset chain can
// only ever shrink it — important when filtering out regions already covered by the cleared area.
const boundShrinkGuard = 3

// Execute clears the regions bounded by paths (all in millimetres), leaving stockToLeave on the
// walls and honouring any already-cleared area. It returns one Output per connected region (helix
// centre, start point, and the adaptive/link toolpaths). stockPaths bounds the raw stock (used to
// skip finishing cuts in thin air); clearedPaths is material already removed. Exact port of
// Adaptive2d::Execute for the clearing op types.
func Execute(cfg Config, stockPaths, paths, clearedPaths []DPath) ([]Output, error) {
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

	// 3) Tag every input path Z=1: it is a real profile wall that wants a finishing pass. The
	// stock-overshoot (step 5) branch adds Z=0 paths for stock boundaries that must NOT be finished;
	// the Z tag is what tells the two apart through the per-curve offset (step 6) and the finishing
	// filter (step 8).
	tagZ(inputPaths, 1)

	// 4) Profiling op types turn each profile curve into a band area between the profile (Z=1) and
	// its 2–3 tool-diameter offset (Z=0), so the same region loop that clears a pocket rides the
	// band instead.
	if cfg.OpType == ProfilingInside || cfg.OpType == ProfilingOutside {
		inputPaths, err = s.profileToAreas(inputPaths)
		if err != nil {
			return nil, err
		}
	}

	// 5) When overshooting the stock is allowed (the upstream default), union the stock-overshoot
	// region (tagged Z=0) into both the input paths and the cleared area so outside clearing is
	// bounded to a frame around the stock instead of running unbounded.
	if !cfg.ForceInsideOut {
		inputPaths, initialCleared, err = s.applyStockOvershoot(inputPaths, stockInputPaths, initialCleared)
		if err != nil {
			return nil, err
		}
	}

	// 6) Tool bounds: offset each input path in by (toolRadius + finishOffset), carrying the Z tag so
	// the finishing filter downstream can see which bounds came from a real wall.
	toolBounds, err := perCurveOffset(inputPaths, inputPaths, -float64(s.toolRadiusScaled+s.finishPassOffsetScaled), false, true)
	if err != nil {
		return nil, err
	}
	return s.clearRegions(toolBounds, stockInputPaths, initialCleared)
}

// applyStockOvershoot ports Execute step 5 (the !forceInsideOut path). It builds the region just
// outside the stock — the reversed, slightly-shrunk stock (stockRev) plus the stock grown outward by
// an overshoot — and unions it into the input paths and the cleared area, tagged Z=0 (a stock
// boundary that needs no finishing). The cleared-area overshoot grows far larger so the tool may
// travel well past the stock edge. Exact port.
func (s *solver) applyStockOvershoot(inputPaths, stockInputPaths, initialCleared clipper.Paths) (clipper.Paths, clipper.Paths, error) {
	stockRev, err := clipper.Offset(stockInputPaths, clipper.Round, clipper.ClosedPolygon, -2, 0, 0)
	if err != nil {
		return nil, nil, err
	}
	clipper.ReversePaths(stockRev)

	overshoot := 4*float64(s.toolRadiusScaled) + s.cfg.StockToLeave*float64(s.scaleFactor)
	outsideInputs, err := clipper.Offset(stockInputPaths, clipper.Square, clipper.ClosedPolygon, overshoot, 0, 0)
	if err != nil {
		return nil, nil, err
	}
	newInputs, err := clipper.Unite(inputPaths, concatPaths(stockRev, outsideInputs))
	if err != nil {
		return nil, nil, err
	}

	outsideCleared, err := clipper.Offset(stockInputPaths, clipper.Square, clipper.ClosedPolygon, 100*float64(s.toolRadiusScaled), 0, 0)
	if err != nil {
		return nil, nil, err
	}
	newCleared, err := clipper.Unite(initialCleared, concatPaths(stockRev, outsideCleared))
	if err != nil {
		return nil, nil, err
	}
	return newInputs, newCleared, nil
}

// concatPaths joins two path sets into one (the two clip sets the upstream adds separately to a
// single union).
func concatPaths(a, b clipper.Paths) clipper.Paths {
	return append(append(clipper.Paths{}, a...), b...)
}

// tagZ stamps the Z tag on every vertex of every path — Execute's "needs finishing" marker (Z=1) or
// the stock-boundary marker (Z=0).
func tagZ(paths clipper.Paths, z int64) {
	for _, p := range paths {
		for i := range p {
			p[i].Z = z
		}
	}
}

// pathHasZ1 reports whether any vertex of the path carries the Z=1 finishing tag.
func pathHasZ1(path clipper.Path) bool {
	for _, p := range path {
		if p.Z == 1 {
			return true
		}
	}
	return false
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
		// 8) Finishing pass: offset only the bounds that came from a real wall (Z=1), skipping the
		// stock-overshoot boundaries, so the finish cut never runs along the artificial stock frame.
		finishingPass, err := perCurveOffset(currentTBP, toolBounds, float64(s.finishPassOffsetScaled), true, false)
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
// (rather than the whole set) matches the upstream, which does it that way because Clipper drops the
// Z tag on a set offset. reference supplies the nesting context.
//
// onlyFinishing skips any path with no Z=1 vertex — the step-8 finishing-pass filter, so a stock
// boundary (all Z=0) is never finished. keepZ1 re-stamps the offset result Z=1 when the source path
// carried the tag — the step-6 tag carry, so the finishing filter can still see it downstream.
func perCurveOffset(paths, reference clipper.Paths, delta float64, onlyFinishing, keepZ1 bool) (clipper.Paths, error) {
	var out clipper.Paths
	for _, path := range paths {
		hasZ1 := pathHasZ1(path)
		if onlyFinishing && !hasZ1 {
			continue
		}
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
			if keepZ1 && hasZ1 {
				tagZ(clipper.Paths{p}, 1)
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
