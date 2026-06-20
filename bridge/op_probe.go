// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
)

// defaultProbeFeed is the slow feed (mm/min) used for probe moves when ProbeFeed is unset.
const defaultProbeFeed = 50.0

// ProbeOp drives a touch probe to find the workpiece — its top and two edges — for setting the
// work-coordinate origin, with G38.2 straight-probe moves. It cuts no material. The probe Points
// are resolved by the engine from the stock bounds (like the 3D ops' rows/levels); Execute is
// pure given them.
type ProbeOp struct {
	OpBase
	ProbeFeed float64          // mm/min — slow feed for the probe moves
	Points    []gen.ProbePoint // resolved by the engine from the stock bounds
}

// Features reports the property groups probing uses (heights for the clearance plane; no tool or
// cut depths — it does not machine).
func (op *ProbeOp) Features() FeatureFlag {
	return FeatureHeights | FeatureBaseGeometry
}

// Execute generates the probing cycle from the resolved probe points at the configured probe
// feed, wrapped in the standard op framing.
func (op *ProbeOp) Execute(_ *Job) (gcode.Path, error) {
	if len(op.Points) == 0 {
		return gcode.Path{}, fmt.Errorf("probe operation %q has no probe points — the engine resolves them from the stock", op.OpLabel)
	}
	cmds, err := gen.GenerateProbe(op.Points, gen.ProbeParams{ClearanceZ: op.ClearanceHeight, ProbeFeed: op.feedRate()})
	if err != nil {
		return gcode.Path{}, fmt.Errorf("probe operation %q: %w", op.OpLabel, err)
	}
	return op.frame(cmds), nil
}

// feedRate returns the configured probe feed, or the default when unset.
func (op *ProbeOp) feedRate() float64 {
	if op.ProbeFeed > 0 {
		return op.ProbeFeed
	}
	return defaultProbeFeed
}
