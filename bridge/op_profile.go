// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
	"oblikovati.org/cam/bridge/geom2d"
)

// ProfileOp cuts a contour around a boundary with tool-radius compensation, stepping down in
// Z. The driving Boundary (a
// closed XY contour in millimetres) is populated by the engine from the part's silhouette /
// a selected face; Execute is pure given the boundary, tool, and depths.
type ProfileOp struct {
	OpBase
	Side           string         // gen.SideOutside | gen.SideInside | gen.SideOn
	OffsetExtra    float64        // extra stock left on the wall (mm)
	Climb          bool           // climb vs conventional milling
	StepDown       float64        // max Z step per pass (mm); <=0 → single pass
	RoughingPasses int            // radial passes to reach the wall (>1 roughs thick stock); 0/1 → single
	RoughStep      float64        // radial step between roughing passes (mm)
	Boundary       geom2d.Polygon // driving contour (mm), populated by the engine
}

// Features reports the property groups a profile uses.
func (op *ProfileOp) Features() FeatureFlag {
	return FeatureTool | FeatureDepths | FeatureHeights | FeatureStepDown | FeatureBaseGeometry
}

// Execute generates the contour toolpath: resolve the tool radius and feeds, build the depth
// levels, and delegate to the profile generator, wrapped in the standard op framing.
func (op *ProfileOp) Execute(job *Job) (gcode.Path, error) {
	tc, err := op.resolveTool(job)
	if err != nil {
		return gcode.Path{}, err
	}
	if len(op.Boundary) < 3 {
		return gcode.Path{}, fmt.Errorf("profile operation %q has no boundary contour", op.OpLabel)
	}
	cmds, err := gen.GenerateProfile(op.Boundary, op.depthLevels(), op.feeds(tc), gen.ProfileParams{
		ToolRadius:     tc.Tool.Diameter / 2,
		Side:           op.Side,
		OffsetExtra:    op.OffsetExtra,
		Climb:          op.Climb,
		RoughingPasses: op.RoughingPasses,
		RoughStep:      op.RoughStep,
	})
	if err != nil {
		return gcode.Path{}, fmt.Errorf("profile operation %q: %w", op.OpLabel, err)
	}
	return op.frame(cmds), nil
}

// depthLevels builds the op's descending cut planes from its depth envelope.
func (op *ProfileOp) depthLevels() []float64 {
	return gen.DepthLevels(op.StartDepth, op.FinalDepth, op.StepDown)
}
