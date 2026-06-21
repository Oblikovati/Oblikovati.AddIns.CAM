// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
	"oblikovati.org/cam/bridge/geom2d"
)

// ChamferOp breaks (bevels) the top edge of a contour with a V-shaped chamfer tool in a single
// pass: the tool flank rides the boundary offset by the chamfer width at a depth set by the
// tool angle, producing a bevel that width wide. It suits deburring and edge-breaking. The
// driving Boundary (a closed XY region in millimetres) is populated by the engine; Execute is
// pure given the boundary, tool, and the top plane.
type ChamferOp struct {
	OpBase
	Width     float64        // mm — horizontal width of the bevel face
	ToolAngle float64        // included angle of the V-tool (degrees); <=0 → 90°
	Side      string         // gen.SideOutside | gen.SideInside | gen.SideOn
	Climb     bool           // climb vs conventional milling
	Passes    int            // flank passes to reach full width (>1 roughs a wide bevel); 0/1 → single
	Boundary  geom2d.Polygon // driving contour (mm), populated by the engine
}

// Features reports the property groups a chamfer uses (no step-down — it is a single pass).
func (op *ChamferOp) Features() FeatureFlag {
	return FeatureTool | FeatureDepths | FeatureHeights | FeatureBaseGeometry
}

// Execute generates the chamfer toolpath: resolve the feeds and delegate to the chamfer
// generator at the start-depth (the top edge), wrapped in the standard op framing.
func (op *ChamferOp) Execute(job *Job) (gcode.Path, error) {
	tc, err := op.resolveTool(job)
	if err != nil {
		return gcode.Path{}, err
	}
	if len(op.Boundary) < 3 {
		return gcode.Path{}, fmt.Errorf("chamfer operation %q has no boundary contour", op.OpLabel)
	}
	feeds := gen.Feeds{Vert: tc.VertFeed, Horiz: tc.HorizFeed, ClearanceZ: op.ClearanceHeight, SafeZ: op.SafeHeight}
	cmds, err := gen.GenerateChamfer(op.Boundary, op.StartDepth, feeds, gen.ChamferParams{
		Width: op.Width, ToolAngleDeg: op.ToolAngle, Side: op.Side, Climb: op.Climb, Passes: op.Passes,
	})
	if err != nil {
		return gcode.Path{}, fmt.Errorf("chamfer operation %q: %w", op.OpLabel, err)
	}
	return op.frame(cmds), nil
}
