// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
	"oblikovati.org/cam/bridge/geom2d"
)

// MillFaceOp faces (clears the top of) the stock over a boundary region with a raster
// pattern, stepping down in Z.
type MillFaceOp struct {
	OpBase
	StepOver float64        // pass spacing as a fraction of the tool diameter (0..1)
	StepDown float64        // max Z step per pass (mm)
	Spiral   bool           // clear with a continuous inward spiral instead of a back-and-forth raster
	Boundary geom2d.Polygon // region to face (mm)
}

// Features reports the property groups face milling uses.
func (op *MillFaceOp) Features() FeatureFlag {
	return FeatureTool | FeatureDepths | FeatureHeights | FeatureStepDown | FeatureBaseGeometry
}

// Execute generates the facing toolpath.
func (op *MillFaceOp) Execute(job *Job) (gcode.Path, error) {
	tc, err := op.resolveTool(job)
	if err != nil {
		return gcode.Path{}, err
	}
	if len(op.Boundary) < 3 {
		return gcode.Path{}, fmt.Errorf("face operation %q has no boundary region", op.OpLabel)
	}
	feeds := op.feeds(tc)
	cmds, err := gen.GenerateMillFace(op.Boundary, gen.DepthLevels(op.StartDepth, op.FinalDepth, op.StepDown), feeds, gen.MillFaceParams{
		ToolRadius: tc.Tool.Diameter / 2,
		StepOver:   op.StepOver,
		Spiral:     op.Spiral,
	})
	if err != nil {
		return gcode.Path{}, fmt.Errorf("face operation %q: %w", op.OpLabel, err)
	}
	return op.frame(cmds), nil
}

// EngraveOp follows a contour with the tool centred on the path (no radius compensation),
// cutting to a single depth — for marking text/outlines. It is a profile with side "on", so
// it reuses the profile generator.
type EngraveOp struct {
	OpBase
	StepDown float64        // max Z step per pass (mm); <=0 → single pass
	Climb    bool           // cut direction
	Boundary geom2d.Polygon // contour to engrave (mm)
}

// Features reports the property groups engraving uses (no tool compensation, so the radius
// only sets the feeds, not an offset).
func (op *EngraveOp) Features() FeatureFlag {
	return FeatureTool | FeatureDepths | FeatureHeights | FeatureStepDown | FeatureBaseGeometry
}

// Execute generates the engraving toolpath: the contour run on the tool centre at each depth
// level. A zero tool radius is allowed (engraving with a V-bit tip is modelled as side "on").
func (op *EngraveOp) Execute(job *Job) (gcode.Path, error) {
	tc, err := op.resolveTool(job)
	if err != nil {
		return gcode.Path{}, err
	}
	if len(op.Boundary) < 3 {
		return gcode.Path{}, fmt.Errorf("engrave operation %q has no boundary contour", op.OpLabel)
	}
	feeds := op.feeds(tc)
	cmds, err := gen.GenerateProfile(op.Boundary, gen.DepthLevels(op.StartDepth, op.FinalDepth, op.StepDown), feeds, gen.ProfileParams{
		ToolRadius: tc.Tool.Diameter / 2,
		Side:       gen.SideOn,
		Climb:      op.Climb,
	})
	if err != nil {
		return gcode.Path{}, fmt.Errorf("engrave operation %q: %w", op.OpLabel, err)
	}
	return op.frame(cmds), nil
}
