// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/geom2d"
)

// PocketParams configure an area-clearing (pocket) pass.
type PocketParams struct {
	ToolRadius float64 // mm
	StepOver   float64 // fraction of the tool diameter to step between rings (0..1); 0 → 0.5
	Climb      bool    // climb vs conventional
}

// defaultStepOver is the ring step (fraction of tool diameter) used when StepOver is unset.
const defaultStepOver = 0.5

// GeneratePocket clears the interior of the boundary with concentric offset rings — the
// boundary offset inward by the tool radius, then repeatedly by the step-over until the
// rings collapse — walked at each depth level from the outermost ring inward. This is the
// client-side area-clearing that stands in for libarea's makePocket (the offset-pattern
// mode); the offset & slicing primitives it would use host-side exist in the API, but the
// clearing pattern itself is add-in logic. See cam-port/gaps.md.
func GeneratePocket(boundary geom2d.Polygon, levels []float64, feeds Feeds, p PocketParams) ([]gcode.Command, error) {
	if p.ToolRadius <= 0 {
		return nil, fmt.Errorf("pocket needs a positive tool radius, got %g", p.ToolRadius)
	}
	rings := pocketRings(boundary, p.ToolRadius, p.stepDistance())
	if len(rings) == 0 {
		return nil, fmt.Errorf("pocket: tool radius %g is too large to enter the region (area %g)", p.ToolRadius, boundary.Area())
	}

	var cmds []gcode.Command
	for _, z := range levels {
		for _, ring := range rings {
			cmds = append(cmds, walkLoop(orient(ring, p.Climb), z, feeds)...)
		}
	}
	return cmds, nil
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
