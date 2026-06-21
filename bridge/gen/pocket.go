// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/geom2d"
)

// PocketParams configure an area-clearing (pocket) pass.
type PocketParams struct {
	ToolRadius      float64          // mm
	StepOver        float64          // fraction of the tool diameter to step between rings (0..1); 0 → 0.5
	Climb           bool             // climb vs conventional
	Islands         []geom2d.Polygon // regions to leave standing (holes/bosses); the clearing routes around them
	FinishAllowance float64          // mm of stock to leave on the walls when roughing; >0 adds a final wall pass
	Pattern         string           // PocketOffset (default) | PocketZigzag
	OneWay          bool             // zigzag only: cut every row the same direction (rapid back) instead of back-and-forth
}

// defaultStepOver is the ring step (fraction of tool diameter) used when StepOver is unset.
const defaultStepOver = 0.5

// GeneratePocket clears the interior of the boundary with concentric offset rings — the
// boundary offset inward by the tool radius, then repeatedly by the step-over until the
// rings collapse — walked at each depth level from the outermost ring inward. This is the
// client-side area-clearing (the offset-pattern mode); the offset & slicing primitives it
// would use host-side exist in the API, but the
// clearing pattern itself is add-in logic.
//
// With a FinishAllowance the roughing rings stop that far short of every wall (outer boundary
// and islands) and a single finishing pass is then run right at the walls, so the rough cut
// leaves a thin even skin the finish pass cleans to size — the standard rough-then-finish split.
func GeneratePocket(boundary geom2d.Polygon, levels []float64, feeds Feeds, p PocketParams) ([]gcode.Command, error) {
	if p.ToolRadius <= 0 {
		return nil, fmt.Errorf("pocket needs a positive tool radius, got %g", p.ToolRadius)
	}
	wallDist := p.ToolRadius + p.FinishAllowance // roughing leaves the allowance on every wall
	if p.Pattern == PocketZigzag {
		return generateZigzagPocket(boundary, levels, feeds, p, wallDist)
	}
	rings := pocketRings(boundary, wallDist, p.stepDistance())
	if len(rings) == 0 {
		return nil, pocketTooSmallErr(p, boundary)
	}
	roughKeepouts := grownIslands(p.Islands, wallDist)
	finishRings, finishKeepouts := pocketFinishPass(boundary, p)

	var cmds []gcode.Command
	for _, z := range levels {
		cmds = append(cmds, walkPocketRings(rings, roughKeepouts, z, feeds, p.Climb)...)
		cmds = append(cmds, walkPocketRings(finishRings, finishKeepouts, z, feeds, p.Climb)...)
	}
	return cmds, nil
}

// pocketTooSmallErr is the shared "tool too large to enter the region" error for both clearing
// patterns. It reports the region's maximum inscribed radius — the largest tool that *would* fit —
// so the message is actionable, not just "too big". Computed only on this error path.
func pocketTooSmallErr(p PocketParams, boundary geom2d.Polygon) error {
	_, fits := geom2d.MaxInscribedCircle(boundary)
	return fmt.Errorf("pocket: tool radius %g (+ allowance %g) exceeds the region's maximum inscribed radius %.3g — too large to enter", p.ToolRadius, p.FinishAllowance, fits)
}

// walkPocketRings walks each ring at depth z routed around the keepouts: a ring clear of every
// keepout is cut as a closed loop, one crossing a keepout is split into the open arc runs that
// stay outside them. Shared by the roughing and finishing passes.
func walkPocketRings(rings, keepouts []geom2d.Polygon, z float64, feeds Feeds, climb bool) []gcode.Command {
	var cmds []gcode.Command
	for _, ring := range rings {
		oriented := orient(ring, climb)
		if len(keepouts) == 0 {
			cmds = append(cmds, walkLoop(oriented, z, feeds)...)
			continue
		}
		for _, run := range clipRingAroundIslands(oriented, keepouts) {
			cmds = append(cmds, walkOpenPath(run, z, feeds)...)
		}
	}
	return cmds
}

// pocketFinishPass builds the finishing rings (right at every wall) and their keepouts, or nil
// when no FinishAllowance was asked for. The outer wall is the boundary offset in by one radius;
// each island wall is the island offset out by one radius. The finish rings route around the
// islands grown by just the radius (the walls they are cleaning to).
func pocketFinishPass(boundary geom2d.Polygon, p PocketParams) (rings, keepouts []geom2d.Polygon) {
	if p.FinishAllowance <= 0 {
		return nil, nil
	}
	if wall, ok := geom2d.Offset(boundary, -p.ToolRadius); ok {
		rings = append(rings, wall)
	}
	rings = append(rings, islandFinishRings(p.Islands, p.ToolRadius)...)
	return rings, grownIslands(p.Islands, p.ToolRadius)
}

// islandFinishRings returns one ring hugging each island wall (the island offset outward by the
// tool radius), the finishing pass around the islands.
func islandFinishRings(islands []geom2d.Polygon, toolRadius float64) []geom2d.Polygon {
	var out []geom2d.Polygon
	for _, isl := range islands {
		if len(isl) < 3 {
			continue
		}
		if ring, ok := geom2d.Offset(isl.EnsureCCW(), toolRadius); ok {
			out = append(out, ring)
		}
	}
	return out
}

// grownIslands offsets each island outward by the tool radius, so the tool centre — clearing
// rings route around these — stays clear of the island walls. Islands that fail to offset are
// dropped.
func grownIslands(islands []geom2d.Polygon, toolRadius float64) []geom2d.Polygon {
	var out []geom2d.Polygon
	for _, isl := range islands {
		if len(isl) < 3 {
			continue
		}
		if grown, ok := geom2d.Offset(isl.EnsureCCW(), toolRadius); ok {
			out = append(out, grown)
		}
	}
	return out
}

// clipRingAroundIslands clips a clearing ring (a closed loop) so it routes around every island,
// returning the arc runs that lie outside all of them. A ring clear of the islands comes back as
// the whole loop.
func clipRingAroundIslands(ring geom2d.Polygon, keepouts []geom2d.Polygon) [][]geom2d.Point2 {
	runs := [][]geom2d.Point2{closeLoop(ring)}
	for _, isl := range keepouts {
		var next [][]geom2d.Point2
		for _, run := range runs {
			next = append(next, geom2d.ClipOutside(run, isl)...)
		}
		runs = next
	}
	return runs
}

// closeLoop returns the polygon's points with the first repeated at the end, so it clips as a
// closed path.
func closeLoop(poly geom2d.Polygon) []geom2d.Point2 {
	return append(append([]geom2d.Point2{}, poly...), poly[0])
}

// stepDistance is the spacing between concentric rings in millimetres (step-over fraction ×
// tool diameter), defaulting to half the diameter when unset.
func (p PocketParams) stepDistance() float64 {
	frac := p.StepOver
	if frac <= 0 {
		frac = defaultStepOver
	}
	return frac * 2 * p.ToolRadius
}

// pocketRings builds the concentric clearing rings from the outermost (boundary offset in by
// one radius, so the tool wall just touches the boundary) inward by spacing each time, until
// an offset collapses (the centre is reached).
func pocketRings(boundary geom2d.Polygon, radius, spacing float64) []geom2d.Polygon {
	var rings []geom2d.Polygon
	for d := radius; ; d += spacing {
		ring, ok := geom2d.Offset(boundary, -d)
		if !ok {
			break
		}
		rings = append(rings, ring)
		if spacing <= 0 { // guard against a non-advancing loop
			break
		}
	}
	return rings
}
