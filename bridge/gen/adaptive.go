// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"fmt"
	"math"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/geom2d"
)

// AdaptiveParams configure HSM (high-speed-machining) adaptive clearing: the region is cleared
// with a low, near-constant radial engagement so the tool can run fast feeds without overload.
// StepOver is that radial engagement as a fraction of the tool diameter (0..1) — small by design
// (the HSM signature), defaulting to defaultAdaptiveStepOver.
type AdaptiveParams struct {
	ToolRadius float64
	StepOver   float64
	Climb      bool
}

// defaultAdaptiveStepOver is the radial engagement (fraction of tool diameter) used when
// StepOver is unset — 10%, an order of magnitude below a roughing pocket's, which is what lets
// the controller hold a high feed.
const defaultAdaptiveStepOver = 0.1

// GenerateAdaptive clears the boundary interior with a continuous inward spiral of low-engagement
// passes: the boundary is offset inward by a small step-over into concentric rings, and the rings
// are linked into ONE stay-down spiral per depth level (the tool never retracts to clear between
// rings, unlike the offset-pattern pocket). The small step-over bounds the radial engagement on
// straight runs; sharp internal corners still spike (a full constant-engagement solver would add
// trochoidal loops there, which needs a medial-axis the add-in's polygon offsetter does not
// provide — see cam-port/gaps.md). This is the HSM analogue of FreeCAD's Path/Op/Adaptive.
func GenerateAdaptive(boundary geom2d.Polygon, levels []float64, feeds Feeds, p AdaptiveParams) ([]gcode.Command, error) {
	if p.ToolRadius <= 0 {
		return nil, fmt.Errorf("adaptive clearing needs a positive tool radius, got %g", p.ToolRadius)
	}
	rings := pocketRings(boundary, p.ToolRadius, p.stepDistance())
	if len(rings) == 0 {
		return nil, fmt.Errorf("adaptive clearing: tool radius %g is too large to enter the region (area %g)", p.ToolRadius, boundary.Area())
	}
	var cmds []gcode.Command
	for _, z := range levels {
		cmds = append(cmds, walkSpiral(rings, z, feeds, p.Climb)...)
	}
	return cmds, nil
}

// stepDistance is the spacing between spiral passes in millimetres (step-over fraction × tool
// diameter), defaulting to defaultAdaptiveStepOver of the diameter when unset.
func (p AdaptiveParams) stepDistance() float64 {
	frac := p.StepOver
	if frac <= 0 {
		frac = defaultAdaptiveStepOver
	}
	return frac * 2 * p.ToolRadius
}

// walkSpiral links the rings (outermost first) into one continuous clearing pass at depth z:
// rapid in and plunge once at the outer ring's start, then feed around each ring and step
// straight inward to the next without retracting, and finally retract once. Each ring is wound
// for the cut direction and entered at its vertex nearest the current position so the inward
// transition is short.
func walkSpiral(rings []geom2d.Polygon, z float64, feeds Feeds, climb bool) []gcode.Command {
	first := orient(rings[0], climb)
	if len(first) < 2 {
		return nil
	}
	cmds := []gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"Z": feeds.ClearanceZ}),
		gcode.NewCommand("G0", map[string]float64{"X": first[0].X, "Y": first[0].Y}),
		gcode.NewCommand("G0", map[string]float64{"Z": feeds.SafeZ}),
		gcode.NewCommand("G1", map[string]float64{"Z": z, "F": feeds.Vert}),
	}
	pos := first[0]
	for i, ring := range rings {
		loop := orient(ring, climb)
		if len(loop) < 2 {
			continue
		}
		if i > 0 {
			loop = rotatedToNearest(loop, pos)                  // enter where closest
			cmds = append(cmds, feedMove(loop[0], feeds.Horiz)) // short inward step
		}
		cmds = append(cmds, feedRing(loop, feeds.Horiz)...)
		pos = loop[0]
	}
	return append(cmds, gcode.NewCommand("G0", map[string]float64{"Z": feeds.ClearanceZ}))
}

// feedRing feeds around one closed loop at the horizontal feed from loop[0] (where the tool
// already is, by plunge or inward step) through the remaining vertices and back to loop[0].
// Stays at the current Z — no plunge, no retract.
func feedRing(loop geom2d.Polygon, horiz float64) []gcode.Command {
	cmds := make([]gcode.Command, 0, len(loop))
	for _, pt := range loop[1:] {
		cmds = append(cmds, feedMove(pt, horiz))
	}
	return append(cmds, feedMove(loop[0], horiz))
}

// feedMove is one G1 XY cutting move at the horizontal feed.
func feedMove(pt geom2d.Point2, horiz float64) gcode.Command {
	return gcode.NewCommand("G1", map[string]float64{"X": pt.X, "Y": pt.Y, "F": horiz})
}

// rotatedToNearest returns the loop re-indexed to begin at its vertex nearest pt, preserving its
// winding, so the inward step from the previous ring is the shortest available.
func rotatedToNearest(loop geom2d.Polygon, pt geom2d.Point2) geom2d.Polygon {
	best, bestD := 0, math.Inf(1)
	for i, v := range loop {
		if d := math.Hypot(v.X-pt.X, v.Y-pt.Y); d < bestD {
			best, bestD = i, d
		}
	}
	if best == 0 {
		return loop
	}
	return append(append(geom2d.Polygon{}, loop[best:]...), loop[:best]...)
}
