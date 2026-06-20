// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
)

// WaterlineOp finishes a 3D surface with constant-Z (waterline) passes — ideal for steep walls
// where parallel passes thin out. The level contours are obtained by slicing the drop-cutter
// surface at each Z (OpenCAMLib for the surface, marching squares for the slices) and handed to
// the op as Levels; Execute shapes them into a framed toolpath.
type WaterlineOp struct {
	OpBase
	StepOver float64          // grid sampling / contour spacing (mm)
	StepDown float64          // Z spacing between levels (mm)
	Levels   []gen.LevelLoops // resolved by the engine from the part mesh
}

// Features reports the property groups a waterline op uses.
func (op *WaterlineOp) Features() FeatureFlag {
	return FeatureTool | FeatureDepths | FeatureHeights | FeatureStepDown | FeatureBaseGeometry
}

// Execute shapes the precomputed level contours into a z-level finishing toolpath.
func (op *WaterlineOp) Execute(job *Job) (gcode.Path, error) {
	tc, err := op.resolveTool(job)
	if err != nil {
		return gcode.Path{}, err
	}
	if len(op.Levels) == 0 {
		return gcode.Path{}, fmt.Errorf("waterline operation %q has no level contours — the engine resolves them from the part mesh", op.OpLabel)
	}
	cmds, err := gen.GenerateWaterline(op.Levels, op.feeds(tc), gen.WaterlineParams{ClearanceZ: op.ClearanceHeight})
	if err != nil {
		return gcode.Path{}, fmt.Errorf("waterline operation %q: %w", op.OpLabel, err)
	}
	return op.frame(cmds), nil
}

// feeds packs the op's clearance height and the controller's feeds for the generator.
func (op *WaterlineOp) feeds(tc ToolController) gen.Feeds {
	return gen.Feeds{Vert: tc.VertFeed, Horiz: tc.HorizFeed, ClearanceZ: op.ClearanceHeight, SafeZ: op.SafeHeight}
}
