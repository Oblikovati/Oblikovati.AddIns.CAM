// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
)

// SurfaceOp finishes a 3D surface with a ball-nose end mill running parallel passes. The
// surface-following Z of each pass is computed by the drop-cutter (OpenCAMLib) over the part
// mesh and handed to the op as Rows; Execute shapes them into a framed toolpath. Mirrors the
// 3D surface-finishing toolpath (the Z-projection comes from OCL).
type SurfaceOp struct {
	OpBase
	StepOver  float64           // distance between parallel scan lines (mm)
	Sampling  float64           // point spacing along a scan line (mm)
	Zigzag    bool              // alternate pass direction
	Rows      [][]gcode.Vector3 // drop-cutter cutter-location rows (mm), populated by the engine
	CrossRows [][]gcode.Vector3 // optional perpendicular rows for a crosshatch finish, populated by the engine
}

// Features reports the property groups a 3D surface op uses.
func (op *SurfaceOp) Features() FeatureFlag {
	return FeatureTool | FeatureDepths | FeatureHeights | FeatureBaseGeometry
}

// Execute shapes the precomputed drop-cutter rows into a parallel finishing toolpath.
func (op *SurfaceOp) Execute(job *Job) (gcode.Path, error) {
	tc, err := op.resolveTool(job)
	if err != nil {
		return gcode.Path{}, err
	}
	if len(op.Rows) == 0 {
		return gcode.Path{}, fmt.Errorf("surface operation %q has no drop-cutter rows — the engine resolves them from the part mesh", op.OpLabel)
	}
	params := gen.SurfaceFinishParams{ClearanceZ: op.ClearanceHeight, Zigzag: op.Zigzag}
	cmds, err := gen.GenerateSurfaceFinish(op.Rows, op.feeds(tc), params)
	if err != nil {
		return gcode.Path{}, fmt.Errorf("surface operation %q: %w", op.OpLabel, err)
	}
	// A crosshatch finish follows the parallel passes with a perpendicular set for a finer scallop.
	if len(op.CrossRows) > 0 {
		cross, err := gen.GenerateSurfaceFinish(op.CrossRows, op.feeds(tc), params)
		if err != nil {
			return gcode.Path{}, fmt.Errorf("surface operation %q crosshatch: %w", op.OpLabel, err)
		}
		cmds = append(cmds, cross...)
	}
	return op.frame(cmds), nil
}

// feeds packs the op's clearance height and the controller's feeds for the generator.
func (op *SurfaceOp) feeds(tc ToolController) gen.Feeds {
	return gen.Feeds{Vert: tc.VertFeed, Horiz: tc.HorizFeed, ClearanceZ: op.ClearanceHeight, SafeZ: op.SafeHeight}
}
