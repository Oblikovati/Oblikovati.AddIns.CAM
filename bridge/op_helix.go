// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
)

// HelixOp bores circular holes larger than the tool by helical interpolation, descending
// from the stock top to the hole bottom. It reuses the part's detected cylindrical holes
// (like Drilling) but cuts each with a helix instead of a canned cycle, so it suits holes
// wider than the drill/end-mill. Ports the role of FreeCAD's Path/Op/Helix.
type HelixOp struct {
	OpBase
	HoleRadius float64 // mm — radius of the hole to bore (the wall the helix follows minus tool)
	Pitch      float64 // mm — vertical drop per turn
	Direction  string  // gen.HelixCW | gen.HelixCCW
	Holes      []DrillTarget
}

// Features reports the property groups helix boring uses.
func (op *HelixOp) Features() FeatureFlag {
	return FeatureTool | FeatureDepths | FeatureHeights | FeatureBaseGeometry
}

// Execute generates the helical-bore toolpath for each hole: a helix of radius
// (HoleRadius − toolRadius) descending from the hole top to its bottom, retracting to the
// clearance plane between holes.
func (op *HelixOp) Execute(job *Job) (gcode.Path, error) {
	tc, err := op.resolveTool(job)
	if err != nil {
		return gcode.Path{}, err
	}
	if len(op.Holes) == 0 {
		return gcode.Path{}, fmt.Errorf("helix operation %q has no holes to bore", op.OpLabel)
	}
	toolR := tc.Tool.Diameter / 2
	outerR := op.HoleRadius - toolR
	if outerR <= 0 {
		return gcode.Path{}, fmt.Errorf("helix operation %q: tool ⌀%g too large for hole radius %g", op.OpLabel, tc.Tool.Diameter, op.HoleRadius)
	}

	var cutting []gcode.Command
	for _, h := range orderedHoles(op.Holes) {
		cmds, err := gen.GenerateHelix(
			gcode.Vector3{X: h.X, Y: h.Y, Z: h.Top}, gcode.Vector3{X: h.X, Y: h.Y, Z: h.Bottom},
			gen.HelixParams{
				OuterRadius: outerR, Pitch: op.Pitch, ToolDiameter: tc.Tool.Diameter,
				RetractHeight: op.ClearanceHeight, Direction: op.Direction,
				StartAt: gen.StartOutside, FinishCircle: true, RampAngleRad: 1.5707963267948966, // π/2 (vertical)
			})
		if err != nil {
			return gcode.Path{}, fmt.Errorf("helix operation %q, hole at (%g,%g): %w", op.OpLabel, h.X, h.Y, err)
		}
		cutting = append(cutting, cmds...)
	}
	return op.frame(cutting), nil
}
