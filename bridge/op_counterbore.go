// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
)

// CounterboreOp cuts a flat-bottom cylindrical recess at the top of each detected hole so a
// socket-head screw seats flush. It reuses the part's holes (like Drilling/Helix) but clears a
// shallow recess of a set diameter and depth at the hole top rather than boring through. Ports
// a counterbore / spot-face.
type CounterboreOp struct {
	OpBase
	Diameter float64 // mm — recess diameter
	Depth    float64 // mm — recess depth below the hole top
	Pitch    float64 // mm — helix drop per turn
	Holes    []DrillTarget
}

// Features reports the property groups counterboring uses.
func (op *CounterboreOp) Features() FeatureFlag {
	return FeatureTool | FeatureDepths | FeatureHeights | FeatureBaseGeometry
}

// Execute clears a flat-bottom recess at each hole top: a helical annulus of the recess diameter
// from the hole top down by the recess depth, retracting to the clearance plane between holes.
func (op *CounterboreOp) Execute(job *Job) (gcode.Path, error) {
	tc, err := op.resolveTool(job)
	if err != nil {
		return gcode.Path{}, err
	}
	if len(op.Holes) == 0 {
		return gcode.Path{}, fmt.Errorf("counterbore operation %q has no holes to spot-face", op.OpLabel)
	}
	var cutting []gcode.Command
	for _, h := range orderedHoles(op.Holes) {
		cmds, err := gen.GenerateCounterbore(
			gcode.Vector3{X: h.X, Y: h.Y, Z: h.Top}, gcode.Vector3{X: h.X, Y: h.Y, Z: h.Top - op.Depth},
			gen.CounterboreParams{Diameter: op.Diameter, ToolDiameter: tc.Tool.Diameter, Pitch: op.Pitch},
		)
		if err != nil {
			return gcode.Path{}, fmt.Errorf("counterbore operation %q, hole at (%g,%g): %w", op.OpLabel, h.X, h.Y, err)
		}
		cutting = append(cutting, cmds...)
	}
	return op.frame(cutting), nil
}
