// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"errors"
	"fmt"
	"math"

	"oblikovati.org/cam/bridge/gcode"
)

// Helix direction + start constants.
const (
	HelixCW      = "CW"
	HelixCCW     = "CCW"
	StartInside  = "Inside"
	StartOutside = "Outside"
)

// HelixParams configure a helical bore: descend into a hole larger than the tool along a
// helix, optionally clearing an annulus (outer→inner radius) in concentric helices.
type HelixParams struct {
	OuterRadius   float64 // mm — the outermost helix radius (tool-centre path)
	InnerRadius   float64 // mm — innermost radius for annulus clearing; <=0 → single helix at OuterRadius
	Pitch         float64 // mm — vertical drop per full turn (cap)
	Step          float64 // mm — radial spacing between concentric helices; 0 → single helix
	ToolDiameter  float64 // mm — used only to size the wall-clearing retract
	RetractHeight float64 // mm — height to move between helices; raised to top if below it
	Direction     string  // HelixCW (G2) | HelixCCW (G3)
	StartAt       string  // StartInside | StartOutside
	FinishCircle  bool    // add a flat finishing circle at the bottom
	RampAngleRad  float64 // ramp angle (rad), 0<θ≤π/2; caps the per-turn descent
}

// GenerateHelix produces the helical-bore toolpath between the hole's top and bottom centre
// points (a vertical edge). It is a port of FreeCAD's helix generator (the vertical, non-cone
// path): each full turn is two 180° arcs, descending linearly, optionally over an annulus of
// concentric helices, each followed by a wall-clearing retract. Emits G2/G3 arcs with I/J
// centre offsets.
func GenerateHelix(top, bottom gcode.Vector3, p HelixParams) ([]gcode.Command, error) {
	if err := validateHelix(top, bottom, p); err != nil {
		return nil, err
	}
	inner := p.InnerRadius
	if inner <= 0 {
		inner = p.OuterRadius
	}
	retract := p.RetractHeight
	if retract < top.Z {
		retract = top.Z
	}
	radii := helixRadii(p.OuterRadius, inner, p.Step, p.StartAt)

	h := helixState{top: top, bottom: bottom, height: top.Z - bottom.Z, retractZ: retract,
		toolRadius: p.ToolDiameter / 2, pitch: p.Pitch, ramp: p.RampAngleRad,
		arc: arcCode(p.Direction), finishCircle: p.FinishCircle, startAt: p.StartAt}

	cmds := []gcode.Command{gcode.NewCommand("G0", map[string]float64{"Z": retract})}
	for i, r := range radii {
		cmds = append(cmds, h.vertical(r)...)
		lastStep := 0.0
		if i > 0 {
			lastStep = math.Abs(radii[i] - radii[i-1])
		}
		cmds = append(cmds, h.retract(r, lastStep))
	}
	return cmds, nil
}

// helixState carries the resolved helix parameters through the per-radius generation.
type helixState struct {
	top, bottom             gcode.Vector3
	height, retractZ        float64
	toolRadius, pitch, ramp float64
	arc                     string
	finishCircle            bool
	startAt                 string
}

// vertical returns the commands of one simple helix at radius r: lead-in, then two 180° arcs
// per turn descending linearly, then an optional flat finishing circle.
func (h helixState) vertical(r float64) []gcode.Command {
	depthPerCircle := math.Min(2*math.Pi*r*math.Tan(h.ramp), h.pitch)
	turns := int(math.Ceil(round6(h.height / depthPerCircle)))
	zsteps := linspace(h.top.Z, h.bottom.Z, 2*turns+1)
	dx := r // dir-angle 0: the helix runs along +X

	cmds := []gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"X": h.top.X + dx, "Y": h.top.Y}),
		gcode.NewCommand("G1", map[string]float64{"Z": h.top.Z}),
	}
	for i := 1; i <= turns; i++ {
		cmds = append(cmds, h.halfArc(-dx, zsteps[2*i-1]), h.halfArc(dx, zsteps[2*i]))
	}
	if h.finishCircle {
		cmds = append(cmds, h.halfArc(-dx, h.bottom.Z), h.halfArc(dx, h.bottom.Z))
	}
	return cmds
}

// halfArc is one 180° arc ending at centre+dx (Y unchanged) at depth z. dx is both the
// endpoint's X offset from the axis and, since the arc starts diametrically opposite at -dx,
// the I centre offset measured from that start point.
func (h helixState) halfArc(dx, z float64) gcode.Command {
	return gcode.NewCommand(h.arc, map[string]float64{
		"X": h.top.X + dx, "Y": h.top.Y, "Z": z, "I": dx, "J": 0,
	})
}

// retract returns the move back to the retract height after one helix, nudged off the wall
// for the centre-clearing or step-over cases (mirrors the upstream retract()).
func (h helixState) retract(r, lastStep float64) gcode.Command {
	offset := 0.0
	switch {
	case lastStep == 0 && r <= h.toolRadius:
		offset = -math.Min(h.toolRadius/2, r)
	case lastStep != 0 && h.startAt == StartInside:
		offset = -math.Min(h.toolRadius/2, lastStep/2)
	case lastStep != 0 && h.startAt == StartOutside && r > h.toolRadius:
		offset = math.Min(h.toolRadius/2, lastStep/2)
	}
	if offset == 0 {
		return gcode.NewCommand("G0", map[string]float64{"Z": h.retractZ})
	}
	return gcode.NewCommand("G0", map[string]float64{"X": h.bottom.X + r + offset, "Y": h.bottom.Y, "Z": h.retractZ})
}

// helixRadii returns the concentric helix radii from outer to inner (single-element for a
// plain helix), reversed when starting from the inside.
func helixRadii(outer, inner, step float64, startAt string) []float64 {
	var radii []float64
	if outer < inner || isClose(outer, inner) || step == 0 {
		radii = []float64{outer}
	} else {
		nr := int(math.Ceil(round6((outer-inner)/step))) + 1
		radii = linspace(outer, inner, nr)
	}
	if startAt == StartInside {
		reverseFloats(radii)
	}
	return radii
}

// arcCode maps the cut direction to its arc G-code (G2 clockwise, G3 counter-clockwise).
func arcCode(direction string) string {
	if direction == HelixCCW {
		return "G3"
	}
	return "G2"
}

// validateHelix rejects illegal geometry and parameters, mirroring the upstream guards.
func validateHelix(top, bottom gcode.Vector3, p HelixParams) error {
	if !isClose(top.X-bottom.X, 0) || !isClose(top.Y-bottom.Y, 0) {
		return fmt.Errorf("helix edge is not aligned with Z axis: top=%v bottom=%v", top, bottom)
	}
	if top.Z < bottom.Z {
		return errors.New("helix start point is below end point")
	}
	if p.OuterRadius <= 0 {
		return fmt.Errorf("helix outer_radius must be > 0, got %g", p.OuterRadius)
	}
	if p.Pitch <= 0 {
		return fmt.Errorf("helix pitch must be > 0, got %g", p.Pitch)
	}
	if p.RampAngleRad <= 0 || p.RampAngleRad > math.Pi/2 {
		return fmt.Errorf("helix ramp angle must be in (0, π/2], got %g", p.RampAngleRad)
	}
	if p.Step < 0 {
		return fmt.Errorf("helix step must be >= 0, got %g", p.Step)
	}
	return nil
}

// linspace returns n evenly spaced values from a to b inclusive (just [a] when n<=1).
func linspace(a, b float64, n int) []float64 {
	if n <= 1 {
		return []float64{a}
	}
	out := make([]float64, n)
	step := (b - a) / float64(n-1)
	for i := range out {
		out[i] = a + step*float64(i)
	}
	return out
}

// reverseFloats reverses a slice in place.
func reverseFloats(s []float64) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// round6 rounds to 6 decimals, matching the upstream round(x, 6) used before the ceil so the
// turn count lands identically.
func round6(x float64) float64 { return math.Round(x*1e6) / 1e6 }
