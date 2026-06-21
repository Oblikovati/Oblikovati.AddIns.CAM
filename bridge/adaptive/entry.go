// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import (
	"fmt"

	"oblikovati.org/cam/bridge/clipper"
)

// entryResult is where adaptive clearing of a region begins: the helix-ramp centre, the largest
// helix radius that fits, and the tool's start position and direction once the ramp has bored in.
type entryResult struct {
	entryPoint        clipper.IntPoint
	toolPos           clipper.IntPoint
	toolDir           DoublePoint
	helixRadiusScaled int64
	found             bool
}

// findEntryPoint locates where to plunge into a region and sizes the entry helix. It offsets the
// not-yet-cleared region inward as far as it can and takes the centroid of the deepest remaining
// island as the helix centre, verifies a minimum helix fits without crossing the boundary, then
// binary-searches the largest helix radius that still fits and seeds the cleared area with that
// helix disc. If a symmetric region puts the candidate in a hole, it clips to one quadrant to
// break symmetry and retries (up to 10 times).
//
// Exact port of Adaptive2d::FindEntryPoint. toolBoundPaths is the region the tool centre may
// occupy (boundary inset by the tool radius); boundPaths is the true boundary. On success it
// seeds cleared with the helix disc. Needs the cgo engine; the progress-visualisation of the
// helix is display-only and omitted.
func (s *solver) findEntryPoint(toolBoundPaths, boundPaths clipper.Paths, cleared *clearedArea) (entryResult, error) {
	var res entryResult
	checkPaths, err := clipper.Subtract(toolBoundPaths, cleared.cleared())
	if err != nil {
		return res, fmt.Errorf("findEntryPoint difference: %w", err)
	}

	var entryPoint clipper.IntPoint
	var helixDisc clipper.Paths
	found := false

	// checkHelixFit offsets the candidate point into a disc of the tool plus helix radius and
	// reports whether it stays within the boundary; it leaves the disc in helixDisc for seeding.
	checkHelixFit := func(testRadiusScaled int64) (bool, error) {
		disc, err := clipper.Offset(clipper.Paths{{entryPoint}}, clipper.Round, clipper.OpenRound, float64(testRadiusScaled+s.toolRadiusScaled), 0, 0)
		if err != nil {
			return false, fmt.Errorf("findEntryPoint helix disc: %w", err)
		}
		disc = clipper.CleanPolygons(disc, cleanPathTolerance)
		crossing, err := clipper.Subtract(disc, boundPaths)
		if err != nil {
			return false, fmt.Errorf("findEntryPoint helix crossing: %w", err)
		}
		helixDisc = disc
		return len(crossing) == 0, nil
	}

	for iter := 0; iter < 10; iter++ {
		entryPoint, found, err = deepestInteriorPoint(checkPaths)
		if err != nil {
			return res, err
		}

		if found {
			fit, err := checkHelixFit(s.helixRampMinRadiusScaled)
			if err != nil {
				return res, err
			}
			if !fit {
				found = false
			} else {
				res.helixRadiusScaled, err = largestHelixThatFits(s.helixRampMinRadiusScaled, s.helixRampMaxRadiusScaled, checkHelixFit)
				if err != nil {
					return res, err
				}
				if err := cleared.addClearedPaths(helixDisc); err != nil { // helixDisc is at the final size
					return res, err
				}
			}
		}

		if found {
			break
		}
		if checkPaths, err = breakSymmetryQuadrant(checkPaths); err != nil {
			return res, err
		}
	}

	if !found {
		return res, nil // caller flags StartPointNotFound
	}
	res.entryPoint = entryPoint
	res.toolPos = clipper.IntPoint{X: entryPoint.X, Y: entryPoint.Y - res.helixRadiusScaled}
	res.toolDir = DoublePoint{X: 1.0, Y: 0.0}
	res.found = true
	return res, nil
}

// largestHelixThatFits binary-searches the biggest radius in [minSize, maxSize] for which fit
// returns true (both ends already known: minSize fits, maxSize may not). After the search it
// re-runs fit at the chosen size so the caller's disc reflects the final radius.
func largestHelixThatFits(minSize, maxSize int64, fit func(int64) (bool, error)) (int64, error) {
	for minSize < maxSize {
		testSize := (minSize + maxSize + 1) / 2 // always > minSize
		ok, err := fit(testSize)
		if err != nil {
			return 0, err
		}
		if ok {
			minSize = testSize
		} else {
			maxSize = testSize - 1 // always >= minSize
		}
	}
	if _, err := fit(minSize); err != nil {
		return 0, err
	}
	return minSize, nil
}

// deepestInteriorPoint offsets checkPaths inward in MIN_STEP_CLIPPER increments until nothing
// remains, then returns the validated centroid of the last non-empty offset — the point furthest
// from the region's edges (rejected if it fell outside the boundary or inside a hole).
func deepestInteriorPoint(checkPaths clipper.Paths) (clipper.IntPoint, bool, error) {
	var lastValid clipper.Paths
	delta := -1.0
	inc, err := clipper.Offset(checkPaths, clipper.Square, clipper.ClosedPolygon, delta, 0, 0)
	if err != nil {
		return clipper.IntPoint{}, false, fmt.Errorf("deepestInteriorPoint offset: %w", err)
	}
	for len(inc) > 0 {
		if inc, err = clipper.Offset(checkPaths, clipper.Square, clipper.ClosedPolygon, delta, 0, 0); err != nil {
			return clipper.IntPoint{}, false, fmt.Errorf("deepestInteriorPoint offset: %w", err)
		}
		if len(inc) > 0 {
			lastValid = inc
		}
		delta -= minStepClipper
	}
	entry, found := pickDeepestCentroid(lastValid, checkPaths)
	return entry, found, nil
}

// breakSymmetryQuadrant clips checkPaths to its bottom-left quadrant so the next entry search
// lands off the centre of a symmetric region (which can sit on a hole). An empty input is
// returned unchanged (there is nothing left to clip).
func breakSymmetryQuadrant(checkPaths clipper.Paths) (clipper.Paths, error) {
	if !hasPoints(checkPaths) {
		return clipper.Paths{}, nil
	}
	rect := quadrantRect(pathsBounds(checkPaths))
	clipped, err := clipper.Intersect(clipper.Paths{rect}, checkPaths)
	if err != nil {
		return nil, fmt.Errorf("breakSymmetryQuadrant: %w", err)
	}
	return clipped, nil
}
