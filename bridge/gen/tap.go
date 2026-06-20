// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
)

// TapParams are the knobs of one tapping cycle. A tap cuts a thread by feeding in at exactly
// the thread Pitch per spindle revolution, then reversing the spindle and feeding back out, so
// the synchronised feed is the controller's job — the post emits the canned cycle and the
// machine holds feed = Pitch × spindle-rpm. RightHand selects G84 (the common case); a
// left-hand thread is cut with the reverse cycle G74. A positive DwellTime adds a bottom dwell.
type TapParams struct {
	Pitch     float64 // mm per thread (and per spindle revolution); must be positive
	LeftHand  bool    // cut a left-hand thread with the reverse cycle G74 instead of G84
	DwellTime float64 // optional dwell at the bottom of the hole, in seconds
}

// GenerateTap produces the G-code for tapping a single hole, given the hole's top (start) and
// bottom (end) points and the synchronised feed (mm/min = Pitch × spindle-rpm). The edge must
// be Z-aligned and the start must sit above the end, like the drill generator. It emits exactly
// one canned-cycle command:
//
//	right-hand → G84
//	left-hand  → G74
//
// with X/Y/Z/R/F (and P when a dwell is requested). The feed must already equal Pitch × rpm;
// the op layer computes it from the tool controller's spindle speed so the generator stays a
// pure geometry-to-command mapping.
func GenerateTap(start, end gcode.Vector3, feed float64, p TapParams) ([]gcode.Command, error) {
	if err := validateTap(start, end, feed, p); err != nil {
		return nil, err
	}
	params := map[string]float64{
		"X": start.X,
		"Y": start.Y,
		"Z": end.Z,
		"R": start.Z,
		"F": feed,
	}
	name := "G84"
	if p.LeftHand {
		name = "G74"
	}
	if p.DwellTime > 0 {
		params["P"] = p.DwellTime
	}
	return []gcode.Command{gcode.NewCommand(name, params)}, nil
}

// validateTap rejects the illegal geometry and parameters, each message naming the offending
// values: a non-positive pitch or feed, a non-Z-aligned edge, or a start at/below the end.
func validateTap(start, end gcode.Vector3, feed float64, p TapParams) error {
	if p.Pitch <= 0 {
		return fmt.Errorf("tapping needs a positive thread pitch, got %g", p.Pitch)
	}
	if feed <= 0 {
		return fmt.Errorf("tapping needs a positive synchronised feed, got %g", feed)
	}
	if !isClose(start.X-end.X, 0) || !isClose(start.Y-end.Y, 0) {
		return fmt.Errorf("tap edge is not aligned with Z axis: start=%v end=%v (ΔX=%g ΔY=%g)",
			start, end, start.X-end.X, start.Y-end.Y)
	}
	if start.Z <= end.Z {
		return fmt.Errorf("tap start point is not above the end point: start.Z=%g end.Z=%g", start.Z, end.Z)
	}
	return nil
}
