// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
)

// Tool-setter approach gap and probe over-travel (mm): the probe starts this far above the
// setter top and is allowed to travel this far below it before erroring.
const (
	toolSetterGap   = 5.0
	toolProbeReach  = 20.0
	toolLengthLcode = 1 // G10 L1 sets a tool-length offset (L20 sets a work offset)
)

// GenerateToolLengthProbe measures a tool against a fixed tool-setter and sets its length offset:
// rapid over the setter, probe down (G38.2 Z) to touch its top at setterTop, then G10 L1 to set
// the tool's length offset there, and retract. The setter XY and its top height are machine
// constants the operator supplies; toolNumber is the offset register (P).
func GenerateToolLengthProbe(setterX, setterY, setterTop float64, toolNumber int, p ProbeParams) ([]gcode.Command, error) {
	if p.ProbeFeed <= 0 {
		return nil, fmt.Errorf("tool-length probing needs a positive probe feed, got %g", p.ProbeFeed)
	}
	if toolNumber <= 0 {
		return nil, fmt.Errorf("tool-length probing needs a positive tool number, got %d", toolNumber)
	}
	return []gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"Z": p.ClearanceZ}),
		gcode.NewCommand("G0", map[string]float64{"X": setterX, "Y": setterY}),
		gcode.NewCommand("G0", map[string]float64{"Z": setterTop + toolSetterGap}),
		gcode.NewCommand("G38.2", map[string]float64{"Z": setterTop - toolProbeReach, "F": p.ProbeFeed}),
		gcode.NewCommand("G10", map[string]float64{"L": toolLengthLcode, "P": float64(toolNumber), "Z": setterTop}),
		gcode.NewCommand("G0", map[string]float64{"Z": p.ClearanceZ}),
	}, nil
}
