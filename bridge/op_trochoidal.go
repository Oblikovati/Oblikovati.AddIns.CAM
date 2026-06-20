// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
	"oblikovati.org/cam/bridge/geom2d"
)

// TrochoidalOp follows a contour with overlapping circular loops (trochoidal milling) instead
// of a full-width pass, keeping radial engagement low so the controller can hold a high feed —
// good for deep slots and hard material. The driving Boundary (a closed XY region in
// millimetres) is populated by the engine; Execute is pure given the boundary, tool, and depths.
type TrochoidalOp struct {
	OpBase
	LoopRadius float64        // mm — radius of each trochoidal loop
	Advance    float64        // mm — centre spacing along the path between loops
	Side       string         // gen.SideOutside | gen.SideInside | gen.SideOn
	StepDown   float64        // max Z step per pass (mm); <=0 → single pass
	Boundary   geom2d.Polygon // driving contour (mm), populated by the engine
}

// Features reports the property groups a trochoidal op uses.
func (op *TrochoidalOp) Features() FeatureFlag {
	return FeatureTool | FeatureDepths | FeatureHeights | FeatureStepDown | FeatureBaseGeometry
}

// Execute generates the trochoidal toolpath: resolve the tool radius and feeds, build the depth
// levels, and delegate to the trochoidal generator, wrapped in the standard op framing.
func (op *TrochoidalOp) Execute(job *Job) (gcode.Path, error) {
	tc, err := op.resolveTool(job)
	if err != nil {
		return gcode.Path{}, err
	}
	if len(op.Boundary) < 3 {
		return gcode.Path{}, fmt.Errorf("trochoidal operation %q has no boundary contour", op.OpLabel)
	}
	feeds := gen.Feeds{Vert: tc.VertFeed, Horiz: tc.HorizFeed, ClearanceZ: op.ClearanceHeight, SafeZ: op.SafeHeight}
	cmds, err := gen.GenerateTrochoidal(op.Boundary, gen.DepthLevels(op.StartDepth, op.FinalDepth, op.StepDown), feeds, gen.TrochoidalParams{
		ToolRadius: tc.Tool.Diameter / 2,
		LoopRadius: op.LoopRadius,
		Advance:    op.Advance,
		Side:       op.Side,
		Climb:      true,
	})
	if err != nil {
		return gcode.Path{}, fmt.Errorf("trochoidal operation %q: %w", op.OpLabel, err)
	}
	return op.frame(cmds), nil
}
