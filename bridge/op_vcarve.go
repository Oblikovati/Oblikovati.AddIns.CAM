// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
	"oblikovati.org/cam/bridge/geom2d"
)

// VCarveOp carves a region with a V-bit as nested inward contours cut progressively deeper, so
// the groove forms a V cross-section — shallow at the edges, deepest along the spine. It suits
// engraved lettering and decorative reliefs. The driving Boundary (a closed XY region in
// millimetres) is populated by the engine; Execute is pure given the boundary, tool, and the top
// plane.
type VCarveOp struct {
	OpBase
	ToolAngle float64        // included angle of the V-bit (degrees); <=0 → 90°
	StepOver  float64        // contour spacing as a fraction of the tool diameter (0..1); 0 → 0.5
	Boundary  geom2d.Polygon // driving region (mm), populated by the engine
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
	feeds := gen.Feeds{Vert: tc.VertFeed, Horiz: tc.HorizFeed, ClearanceZ: op.ClearanceHeight, SafeZ: op.SafeHeight}
	cmds, err := gen.GenerateVCarve(op.Boundary, op.StartDepth, feeds, gen.VCarveParams{
		ToolAngleDeg: op.ToolAngle,
		ToolDiameter: tc.Tool.Diameter,
		StepOver:     op.StepOver,
	})
	if err != nil {
		return gcode.Path{}, fmt.Errorf("v-carve operation %q: %w", op.OpLabel, err)
	}
	return op.frame(cmds), nil
}
