// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
)

// CountersinkOp cuts a conical recess at the top of each detected hole so a flat-head screw sits
// flush, by spiralling a countersink/V-tool from the rim down to the centre. It reuses the part's
// holes (like Drilling/Helix/Counterbore) but cuts a cone rather than a flat-bottom recess. Ports
// a countersink.
type CountersinkOp struct {
	OpBase
	Diameter  float64 // mm — countersink rim diameter
	ToolAngle float64 // included angle of the V-tool (degrees); <=0 → 90°
	Holes     []DrillTarget
}

// Features reports the property groups countersinking uses.
func (op *CountersinkOp) Features() FeatureFlag {
	return FeatureTool | FeatureDepths | FeatureHeights | FeatureBaseGeometry
}

// Execute cuts a conical countersink at each hole top: an inward-and-down spiral of the rim
// diameter, retracting to the clearance plane between holes.
func (op *CountersinkOp) Execute(job *Job) (gcode.Path, error) {
	tc, err := op.resolveTool(job)
	if err != nil {
		return gcode.Path{}, err
	}
	if len(op.Holes) == 0 {
		return gcode.Path{}, fmt.Errorf("countersink operation %q has no holes to countersink", op.OpLabel)
	}
	var cutting []gcode.Command
	for _, h := range orderedHoles(op.Holes) {
		cmds, err := gen.GenerateCountersink(
			gcode.Vector3{X: h.X, Y: h.Y, Z: h.Top}, tc.HorizFeed*op.feedFactor(),
			gen.CountersinkParams{Diameter: op.Diameter, ToolAngleDeg: op.ToolAngle, ToolDiameter: tc.Tool.Diameter},
		)
		if err != nil {
			return gcode.Path{}, fmt.Errorf("countersink operation %q, hole at (%g,%g): %w", op.OpLabel, h.X, h.Y, err)
		}
		// retract to the clearance plane before moving to the next hole
		cutting = append(cutting, cmds...)
		cutting = append(cutting, gcode.NewCommand("G0", map[string]float64{"Z": op.ClearanceHeight}))
	}
	return op.frame(cutting), nil
}
