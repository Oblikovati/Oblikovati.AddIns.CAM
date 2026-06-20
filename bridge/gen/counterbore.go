// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
)

// CounterboreParams configure a counterbore / spot-face: a flat-bottom cylindrical recess cut at
// the top of a hole so a socket-head screw sits flush. The recess of Diameter is cleared down to
// the cut depth by helical annulus clearing (concentric helices from the wall inward) finished
// with a flat circle, reusing the helix generator.
type CounterboreParams struct {
	Diameter     float64 // mm — recess diameter (> tool diameter)
	ToolDiameter float64 // mm
	Pitch        float64 // mm — helix drop per turn
	StepOver     float64 // fraction of tool diameter between concentric helices (0..1); 0 → 0.5
}

// GenerateCounterbore clears a flat-bottom recess between the top and bottom centre points by
// helical annulus clearing with a finishing circle, so a screw head seats flush. The recess
// diameter must exceed the tool diameter.
func GenerateCounterbore(top, bottom gcode.Vector3, p CounterboreParams) ([]gcode.Command, error) {
	if p.ToolDiameter <= 0 {
		return nil, fmt.Errorf("counterbore needs a positive tool diameter, got %g", p.ToolDiameter)
	}
	toolR := p.ToolDiameter / 2
	outerR := p.Diameter/2 - toolR
	if outerR <= 0 {
		return nil, fmt.Errorf("counterbore diameter %g must exceed the tool diameter %g", p.Diameter, p.ToolDiameter)
	}
	if p.Pitch <= 0 {
		return nil, fmt.Errorf("counterbore needs a positive pitch, got %g", p.Pitch)
	}
	return GenerateHelix(top, bottom, HelixParams{
		OuterRadius:   outerR,
		InnerRadius:   minPositive(toolR/2, outerR), // clear in toward the centre
		Pitch:         p.Pitch,
		Step:          counterboreStep(p.StepOver) * p.ToolDiameter,
		ToolDiameter:  p.ToolDiameter,
		RetractHeight: top.Z,
		Direction:     HelixCW,
		StartAt:       StartOutside,
		FinishCircle:  true,
		RampAngleRad:  1.5707963267948966, // π/2 — a vertical helix (plunge-limited by pitch)
	})
}

// counterboreStep returns the concentric-helix spacing fraction, defaulting to 0.5.
func counterboreStep(frac float64) float64 {
	if frac <= 0 {
		return 0.5
	}
	return frac
}

// minPositive returns the smaller of a and b, but never more than b (keeps the inner radius
// inside the outer one for a narrow recess).
func minPositive(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
