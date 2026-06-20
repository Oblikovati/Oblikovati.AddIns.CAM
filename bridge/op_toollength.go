// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
)

// ToolLengthProbeOp measures a tool against a fixed tool-setter and sets its length offset
// (G10 L1), with a single G38.2 Z probe. It cuts no material. The setter position and top
// height are machine constants the operator supplies (editable); the tool number is the offset
// register.
type ToolLengthProbeOp struct {
	OpBase
	SetterX    float64 // mm — tool-setter X (machine coordinate)
	SetterY    float64 // mm — tool-setter Y
	SetterTop  float64 // mm — Z of the setter's top surface
	ToolNumber int     // tool-length offset register (P)
	ProbeFeed  float64 // mm/min — slow feed for the probe move
}

// Features reports the property groups tool-length probing uses (heights for the clearance
// plane; no cutting tool or depths).
func (op *ToolLengthProbeOp) Features() FeatureFlag {
	return FeatureHeights | FeatureBaseGeometry
}

// Execute generates the tool-length probing cycle at the configured probe feed, wrapped in the
// standard op framing.
func (op *ToolLengthProbeOp) Execute(_ *Job) (gcode.Path, error) {
	if op.ToolNumber <= 0 {
		return gcode.Path{}, fmt.Errorf("tool-length probe %q has no tool number", op.OpLabel)
	}
	cmds, err := gen.GenerateToolLengthProbe(op.SetterX, op.SetterY, op.SetterTop, op.ToolNumber,
		gen.ProbeParams{ClearanceZ: op.ClearanceHeight, ProbeFeed: op.feedRate()})
	if err != nil {
		return gcode.Path{}, fmt.Errorf("tool-length probe %q: %w", op.OpLabel, err)
	}
	return op.frame(cmds), nil
}

// feedRate returns the configured probe feed, or the shared probe default when unset.
func (op *ToolLengthProbeOp) feedRate() float64 {
	if op.ProbeFeed > 0 {
		return op.ProbeFeed
	}
	return defaultProbeFeed
}
