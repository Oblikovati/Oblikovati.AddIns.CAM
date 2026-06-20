// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
)

// ProbePoint is one touch probe: rapid to Approach, then feed in a straight line toward Target
// until the probe trips (G38.2). Target differs from Approach in the probed axis (or axes for a
// diagonal probe). Used to find the workpiece top and edges for setting the work offset.
type ProbePoint struct {
	Approach gcode.Vector3
	Target   gcode.Vector3
}

// ProbeParams configure a probing cycle.
type ProbeParams struct {
	ClearanceZ float64 // mm — rapid/retract plane above the part
	ProbeFeed  float64 // mm/min — slow feed for the probe moves
}

// GenerateProbe emits a touch-probing cycle: for each point, rapid to the clearance plane, over
// the approach XY, down to the approach Z, then G38.2 (probe toward, error on no contact) at the
// probe feed, and retract. The trip coordinate is reported by the controller; this generates the
// motion. A point whose target equals its approach is skipped (no probe direction).
func GenerateProbe(points []ProbePoint, p ProbeParams) ([]gcode.Command, error) {
	if p.ProbeFeed <= 0 {
		return nil, fmt.Errorf("probing needs a positive probe feed, got %g", p.ProbeFeed)
	}
	if len(points) == 0 {
		return nil, fmt.Errorf("probing needs at least one probe point")
	}
	var cmds []gcode.Command
	for i, pt := range points {
		probe, ok := probeMove(pt, p.ProbeFeed)
		if !ok {
			return nil, fmt.Errorf("probe point %d has no probe direction (target == approach)", i)
		}
		cmds = append(cmds,
			gcode.NewCommand("G0", map[string]float64{"Z": p.ClearanceZ}),
			gcode.NewCommand("G0", map[string]float64{"X": pt.Approach.X, "Y": pt.Approach.Y}),
			gcode.NewCommand("G0", map[string]float64{"Z": pt.Approach.Z}),
			probe,
			gcode.NewCommand("G0", map[string]float64{"Z": p.ClearanceZ}),
		)
	}
	return cmds, nil
}

// probeMove builds the G38.2 straight-probe command toward the target, addressing only the axes
// that move. Reports ok=false when no axis moves.
func probeMove(pt ProbePoint, feed float64) (gcode.Command, bool) {
	params := map[string]float64{"F": feed}
	moved := false
	if !isClose(pt.Target.X-pt.Approach.X, 0) {
		params["X"], moved = pt.Target.X, true
	}
	if !isClose(pt.Target.Y-pt.Approach.Y, 0) {
		params["Y"], moved = pt.Target.Y, true
	}
	if !isClose(pt.Target.Z-pt.Approach.Z, 0) {
		params["Z"], moved = pt.Target.Z, true
	}
	if !moved {
		return gcode.Command{}, false
	}
	return gcode.NewCommand("G38.2", params), true
}
