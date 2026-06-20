// SPDX-License-Identifier: GPL-2.0-only

// Package gen holds the pure toolpath generators: functions that turn a small geometric
// input (a hole edge, a helix span, …) into a list of gcode.Commands with no host or
// kernel dependency. They are the algorithmic primitives of the CAM add-in and the most
// directly testable layer. Mirrors FreeCAD's Path/Base/Generator.
package gen

import (
	"errors"
	"fmt"
	"math"

	"oblikovati.org/cam/bridge/gcode"
)

// DrillParams are the knobs of one drilling cycle. Zero values give a plain G81 drill:
// no dwell, no peck, single pass, retract to the start (top) Z.
type DrillParams struct {
	DwellTime     float64  // G82 dwell at the bottom, in seconds (>0 selects G82)
	PeckDepth     float64  // G83/G73 peck increment (>0 selects a peck cycle)
	Repeat        int      // L repeat count (<=1 omits L); the generated cycle repeats
	RetractHeight *float64 // R plane; nil means retract to the start (top) Z
	ChipBreak     bool     // emit G73 (chip-break, small retracts) instead of G83
	FeedRetract   bool     // emit G85 (feed-out, for boring/reaming) instead of G81
}

// drillTolAbs/drillTolRel reproduce numpy.isclose(rtol=1e-05, atol=1e-06): a point is
// "on the Z axis" when its X/Y delta is within atol + rtol*|b| of zero. Same constants as
// FreeCAD's drill generator so the alignment check accepts/rejects identically.
const (
	drillTolAbs = 1e-06
	drillTolRel = 1e-05
)

// GenerateDrill produces the G-code for drilling a single hole, given the hole's top
// (start) and bottom (end) points. The edge must be aligned with the Z axis (X and Y of
// start and end equal within tolerance) and the start must sit above the end. It is a
// straight port of FreeCAD's Path.Base.Generator.drill.generate and emits exactly one
// canned-cycle command:
//
//	plain          → G81
//	with dwell     → G82 (P = dwell seconds)
//	with peck      → G83, or G73 when ChipBreak (Q = peck increment)
//	feed-retract   → G85 (boring/reaming)
//
// Returns an error (rather than the Python ValueError) for the same illegal combinations,
// each message naming the offending values.
func GenerateDrill(start, end gcode.Vector3, p DrillParams) ([]gcode.Command, error) {
	if err := validateDrill(start, end, p); err != nil {
		return nil, err
	}

	params := map[string]float64{
		"X": start.X,
		"Y": start.Y,
		"Z": end.Z,
		"R": start.Z,
	}
	if p.RetractHeight != nil {
		params["R"] = *p.RetractHeight
	}
	if p.Repeat > 1 {
		params["L"] = float64(p.Repeat)
	}

	name := selectDrillCycle(p, params)
	return []gcode.Command{gcode.NewCommand(name, params)}, nil
}

// selectDrillCycle picks the canned-cycle code and adds the cycle-specific address (P for
// dwell, Q for peck), following the same precedence as the upstream generator: feed-retract
// first, then plain/dwell, then peck.
func selectDrillCycle(p DrillParams, params map[string]float64) string {
	switch {
	case p.FeedRetract:
		return "G85"
	case p.PeckDepth == 0.0:
		if p.DwellTime > 0.0 {
			params["P"] = p.DwellTime
			return "G82"
		}
		return "G81"
	default:
		params["Q"] = p.PeckDepth
		if p.ChipBreak {
			return "G73"
		}
		return "G83"
	}
}

// validateDrill rejects the illegal parameter combinations and geometry, mirroring the
// guards in the upstream generator (peck/dwell/feed-retract are mutually exclusive; repeat
// ≥ 1; edge Z-aligned; start above end).
func validateDrill(start, end gcode.Vector3, p DrillParams) error {
	if p.DwellTime > 0.0 && p.PeckDepth > 0.0 {
		return errors.New("peck and dwell cannot be used together")
	}
	if p.DwellTime > 0.0 && p.FeedRetract {
		return errors.New("dwell and feed retract cannot be used together")
	}
	if p.PeckDepth > 0.0 && p.FeedRetract {
		return errors.New("peck and feed retract cannot be used together")
	}
	if p.Repeat < 1 {
		return fmt.Errorf("repeat must be 1 or greater, got %d", p.Repeat)
	}
	if !isClose(start.X-end.X, 0) || !isClose(start.Y-end.Y, 0) {
		return fmt.Errorf("edge is not aligned with Z axis: start=%v end=%v (ΔX=%g ΔY=%g)",
			start, end, start.X-end.X, start.Y-end.Y)
	}
	if start.Z < end.Z {
		return fmt.Errorf("start point is below end point: start.Z=%g end.Z=%g", start.Z, end.Z)
	}
	return nil
}

// isClose reports whether a ≈ b within numpy.isclose's default tolerances.
func isClose(a, b float64) bool {
	return math.Abs(a-b) <= drillTolAbs+drillTolRel*math.Abs(b)
}
