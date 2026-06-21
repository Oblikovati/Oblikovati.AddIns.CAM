// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import (
	"fmt"

	"oblikovati.org/cam/bridge/clipper"
)

// clearedArea is the solver's running model of the region already machined. The engagement
// integration (calcCutArea) queries it for the cleared geometry near the tool, and the cutting
// loop grows it after every pass — so a later pass only counts the freshly removed material.
//
// It is an exact port of the Adaptive2d ClearedArea class. The geometry operations run on the
// vendored clipping engine, so the mutating/query methods need the cgo build and return an error
// otherwise; the plain accessors are pure Go.
//
// The cleanPathTolerance for tidying boolean results is sqrt(2)+ in scaled units, matching the
// solver's CLEAN_PATH_TOLERANCE.
type clearedArea struct {
	toolRadiusScaled int64
	clearedPaths     clipper.Paths
}

// cleanPathTolerance is the solver's path-cleaning proximity (>sqrt(2)); near-duplicate vertices
// the boolean leaves behind are stripped within it.
const cleanPathTolerance = 1.415

// newClearedArea makes an empty cleared-area model for a tool of the given scaled radius.
func newClearedArea(toolRadiusScaled int64) *clearedArea {
	return &clearedArea{toolRadiusScaled: toolRadiusScaled}
}

// setClearedPaths replaces the cleared geometry wholesale (e.g. the initial already-cleared input).
func (c *clearedArea) setClearedPaths(paths clipper.Paths) {
	c.clearedPaths = paths
}

// cleared returns the full cleared geometry.
func (c *clearedArea) cleared() clipper.Paths {
	return c.clearedPaths
}

// addClearedPaths unions more cleared polygons into the model and tidies the result.
func (c *clearedArea) addClearedPaths(paths clipper.Paths) error {
	merged, err := clipper.Unite(c.clearedPaths, paths)
	if err != nil {
		return fmt.Errorf("clearedArea.addClearedPaths: %w", err)
	}
	c.clearedPaths = clipper.CleanPolygons(merged, cleanPathTolerance)
	return nil
}

// expandCleared grows the cleared region by the swept tool footprint of toClearToolPath: the
// open path is offset by the tool radius (+1, to fully cover it) with a round join/cap, then
// unioned into the cleared geometry. This is how a freshly cut pass enlarges the model.
func (c *clearedArea) expandCleared(toClearToolPath clipper.Path) error {
	if len(toClearToolPath) == 0 {
		return nil
	}
	cover, err := clipper.Offset(clipper.Paths{toClearToolPath}, clipper.Round, clipper.OpenRound, float64(c.toolRadiusScaled+1), 0, 0)
	if err != nil {
		return fmt.Errorf("clearedArea.expandCleared offset: %w", err)
	}
	merged, err := clipper.Unite(c.clearedPaths, cover)
	if err != nil {
		return fmt.Errorf("clearedArea.expandCleared union: %w", err)
	}
	c.clearedPaths = clipper.CleanPolygons(merged, cleanPathTolerance)
	return nil
}

// boundedClearedAreaClipped returns the cleared geometry intersected with the square window of
// half-width delta about toolPos — the local cleared geometry calcCutArea integrates against.
// (The upstream class caches an enlarged window between calls; this computes the clip directly,
// which is identical in result. See the cleared-area cache note in cam-port/gaps.md.)
func (c *clearedArea) boundedClearedAreaClipped(toolPos clipper.IntPoint, delta int64) (clipper.Paths, error) {
	window := clipper.Path{
		{X: toolPos.X - delta, Y: toolPos.Y - delta},
		{X: toolPos.X + delta, Y: toolPos.Y - delta},
		{X: toolPos.X + delta, Y: toolPos.Y + delta},
		{X: toolPos.X - delta, Y: toolPos.Y + delta},
	}
	clipped, err := clipper.Intersect(clipper.Paths{window}, c.clearedPaths)
	if err != nil {
		return nil, fmt.Errorf("clearedArea.boundedClearedAreaClipped: %w", err)
	}
	return clipped, nil
}
