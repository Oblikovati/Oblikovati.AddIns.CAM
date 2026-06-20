// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/geom2d"
)

// RestParams configure rest machining — clearing only the stock a previous, larger tool left
// behind. A tool of radius R can bring its centre no closer than R to the wall, so it leaves an
// uncut band hugging the boundary (and the concave corners). The current, smaller tool of radius
// ToolRadius clears that band: the concentric rings at offsets [ToolRadius, PrevRadius) from the
// boundary, which is precisely the wall stock the previous tool could not reach. PrevRadius must
// exceed ToolRadius (the rest tool has to be smaller than the one it follows).
type RestParams struct {
	ToolRadius float64
	PrevRadius float64
	StepOver   float64 // fraction of tool diameter between rings (0..1); 0 → defaultStepOver
	Climb      bool
}

// GenerateRest clears the uncut wall band left by a previous tool of radius PrevRadius: the
// boundary offset inward from one current-tool radius out to (but not reaching) the previous
// tool's radius, walked ring-by-ring at each depth level. The interior, which the previous tool
// already cleared, is left untouched. The offset & ring primitives are the same as the pocket's;
// rest machining differs only in WHICH rings it cuts. This is the add-in analogue of FreeCAD's
// rest-machining option on the clearing operations.
func GenerateRest(boundary geom2d.Polygon, levels []float64, feeds Feeds, p RestParams) ([]gcode.Command, error) {
	if p.ToolRadius <= 0 {
		return nil, fmt.Errorf("rest machining needs a positive tool radius, got %g", p.ToolRadius)
	}
	if p.PrevRadius <= p.ToolRadius {
		return nil, fmt.Errorf("rest machining needs a previous tool larger than the current one: prev radius %g <= current %g", p.PrevRadius, p.ToolRadius)
	}
	rings := restRings(boundary, p.ToolRadius, p.PrevRadius, p.stepDistance())
	if len(rings) == 0 {
		return nil, fmt.Errorf("rest machining: no wall band to clear (tool radius %g, previous %g, area %g)", p.ToolRadius, p.PrevRadius, boundary.Area())
	}
	var cmds []gcode.Command
	for _, z := range levels {
		for _, ring := range rings {
			cmds = append(cmds, walkLoop(orient(ring, p.Climb), z, feeds)...)
		}
	}
	return cmds, nil
}

// stepDistance is the spacing between rings in millimetres (step-over fraction × tool diameter),
// defaulting to defaultStepOver of the diameter when unset.
func (p RestParams) stepDistance() float64 {
	frac := p.StepOver
	if frac <= 0 {
		frac = defaultStepOver
	}
	return frac * 2 * p.ToolRadius
}

// restRings builds the wall-band rings from the current tool's wall pass (offset by one radius,
// so the tool just touches the boundary) inward by the spacing, stopping before the previous
// tool's reach — everything from there inward was already cleared. A collapsing offset (the band
// pinches out in a narrow feature) ends the band early.
func restRings(boundary geom2d.Polygon, radius, prevRadius, spacing float64) []geom2d.Polygon {
	var rings []geom2d.Polygon
	for d := radius; d < prevRadius; d += spacing {
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
