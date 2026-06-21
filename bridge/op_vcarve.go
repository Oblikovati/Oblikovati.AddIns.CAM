// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
	"oblikovati.org/cam/bridge/geom2d"
)

// VCarveOp carves a region with a V-bit along its medial axis, cut deeper where the region is wider,
// so the groove forms a V cross-section — shallow where the walls crowd, deepest along the spine. It
// suits engraved lettering and decorative reliefs. The driving Boundary (a closed XY region in
// millimetres) is populated by the engine; Execute is pure given the boundary, tool, and the top
// plane.
type VCarveOp struct {
	OpBase
	ToolAngle   float64        // included angle of the V-bit (degrees); <=0 → 90°
	TipDiameter float64        // flat tip diameter of the bit (mm); 0 for a sharp point
	StepDown    float64        // max depth per roughing pass (mm); 0 → a single pass to final depth
	Boundary    geom2d.Polygon // driving region (mm), populated by the engine
}

// Features reports the property groups V-carving uses (no step-down — the depth follows the
// distance from the edge, not a fixed pass).
func (op *VCarveOp) Features() FeatureFlag {
	return FeatureTool | FeatureDepths | FeatureHeights | FeatureBaseGeometry
}

// Execute generates the V-carving toolpath from the top plane (the start depth), wrapped in the
// standard op framing.
func (op *VCarveOp) Execute(job *Job) (gcode.Path, error) {
	tc, err := op.resolveTool(job)
	if err != nil {
		return gcode.Path{}, err
	}
	if len(op.Boundary) < 3 {
		return gcode.Path{}, fmt.Errorf("v-carve operation %q has no boundary region", op.OpLabel)
	}
	feeds := op.feeds(tc)
	cmds, err := gen.GenerateVCarve(op.Boundary, op.StartDepth, feeds, gen.VCarveParams{
		ToolAngleDeg: op.ToolAngle,
		ToolDiameter: tc.Tool.Diameter,
		TipDiameter:  op.TipDiameter,
		FinalDepth:   op.FinalDepth,
		StepDown:     op.StepDown,
	})
	if err != nil {
		return gcode.Path{}, fmt.Errorf("v-carve operation %q: %w", op.OpLabel, err)
	}
	return op.frame(cmds), nil
}
