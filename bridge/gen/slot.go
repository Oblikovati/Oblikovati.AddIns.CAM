// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"fmt"
	"math"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/geom2d"
)

// SlotParams configure slot / groove milling: cut a channel of a given Width centred on the
// boundary path. A slot exactly the tool's width is a single pass on the path; a wider slot is
// cleared with extra passes offset symmetrically to either side, spaced by the step-over. Unlike
// a pocket (which clears inward from a boundary), the slot is centred on the path — for O-ring
// grooves, channels, and lettering troughs.
type SlotParams struct {
	ToolRadius float64 // mm
	Width      float64 // mm — full slot width (>= tool diameter)
	StepOver   float64 // fraction of tool diameter between side passes (0..1); 0 → 0.75
	Climb      bool    // climb vs conventional
}

// defaultSlotStepOver is the side-pass spacing (fraction of tool diameter) when StepOver is unset.
const defaultSlotStepOver = 0.75

// GenerateSlot cuts a slot of Width centred on the boundary: the centreline pass plus symmetric
// passes offset to each side out to (Width − toolDiameter)/2, walked at every depth level. A
// width at or below the tool diameter collapses to the single centreline pass.
func GenerateSlot(boundary geom2d.Polygon, levels []float64, feeds Feeds, p SlotParams) ([]gcode.Command, error) {
	if p.ToolRadius <= 0 {
		return nil, fmt.Errorf("slot needs a positive tool radius, got %g", p.ToolRadius)
	}
	if len(boundary) < 3 {
		return nil, fmt.Errorf("slot boundary needs at least 3 points, got %d", len(boundary))
	}
	if p.Width < 2*p.ToolRadius {
		return nil, fmt.Errorf("slot width %g is narrower than the tool diameter %g", p.Width, 2*p.ToolRadius)
	}
	offsets := slotOffsets(p.Width/2-p.ToolRadius, p.stepDistance())
	rings := slotRings(boundary, offsets)
	if len(rings) == 0 {
		return nil, fmt.Errorf("slot: width %g too wide for the feature (every side pass collapses)", p.Width)
	}
	var cmds []gcode.Command
	for _, z := range levels {
		for _, ring := range rings {
			cmds = append(cmds, walkLoop(orient(ring, p.Climb), z, feeds)...)
		}
	}
	return cmds, nil
}

// stepDistance is the spacing between side passes in millimetres (step-over fraction × tool
// diameter), defaulting to defaultSlotStepOver of the diameter.
func (p SlotParams) stepDistance() float64 {
	frac := p.StepOver
	if frac <= 0 {
		frac = defaultSlotStepOver
	}
	return frac * 2 * p.ToolRadius
}

// slotOffsets returns the symmetric pass offsets from −halfClear to +halfClear (inclusive),
// always including the centreline (0). A non-positive halfClear yields just the centreline.
func slotOffsets(halfClear, spacing float64) []float64 {
	if halfClear <= 1e-9 || spacing <= 0 {
		return []float64{0}
	}
	n := int(math.Ceil(2 * halfClear / spacing))
	if n%2 == 1 {
		n++ // keep the centreline (0) on the grid
	}
	offsets := make([]float64, 0, n+1)
	for i := 0; i <= n; i++ {
		offsets = append(offsets, -halfClear+2*halfClear*float64(i)/float64(n))
	}
	return offsets
}

// slotRings offsets the boundary by each pass offset, dropping any that collapse (an inward
// offset that pinches out in a tight feature). The centreline (offset 0) is the boundary itself.
func slotRings(boundary geom2d.Polygon, offsets []float64) []geom2d.Polygon {
	var rings []geom2d.Polygon
	for _, o := range offsets {
		if math.Abs(o) < 1e-9 {
			rings = append(rings, boundary.EnsureCCW())
			continue
		}
		if ring, ok := geom2d.Offset(boundary, o); ok {
			rings = append(rings, ring)
		}
	}
	return rings
}
