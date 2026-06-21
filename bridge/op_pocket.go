// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
	"oblikovati.org/cam/bridge/geom2d"
)

// PocketOp clears the interior of a boundary region with concentric offset rings, stepping
// down in Z. It ports the toolpath behaviour of FreeCAD's Path/Op/Pocket (offset-pattern
// clearing). The driving Boundary (a closed XY region in millimetres) is populated by the
// engine; Execute is pure given the boundary, tool, and depths.
type PocketOp struct {
	OpBase
	StepOver        float64          // ring step as a fraction of the tool diameter (0..1); 0 → 0.5
	Climb           bool             // climb vs conventional milling
	StepDown        float64          // max Z step per pass (mm); <=0 → single pass
	FinishAllowance float64          // mm of stock left on the walls when roughing; >0 adds a finish pass
	Pattern         string           // gen.PocketOffset (default) | gen.PocketZigzag
	OneWay          bool             // zigzag only: one-direction rows (consistent climb) instead of back-and-forth
	Boundary        geom2d.Polygon   // driving region (mm), populated by the engine
	Islands         []geom2d.Polygon // regions to leave standing (holes/bosses); the clearing routes around them
}

// Features reports the property groups a pocket uses.
func (op *PocketOp) Features() FeatureFlag {
	return FeatureTool | FeatureDepths | FeatureHeights | FeatureStepDown | FeatureBaseGeometry
}

// Execute generates the area-clearing toolpath: resolve the tool radius and feeds, build the
// depth levels, and delegate to the pocket generator, wrapped in the standard op framing.
func (op *PocketOp) Execute(job *Job) (gcode.Path, error) {
	tc, err := op.resolveTool(job)
	if err != nil {
		return gcode.Path{}, err
	}
	if len(op.Boundary) < 3 {
		return gcode.Path{}, fmt.Errorf("pocket operation %q has no boundary region", op.OpLabel)
	}
	feeds := gen.Feeds{Vert: tc.VertFeed, Horiz: tc.HorizFeed, ClearanceZ: op.ClearanceHeight, SafeZ: op.SafeHeight}
	cmds, err := gen.GeneratePocket(op.Boundary, gen.DepthLevels(op.StartDepth, op.FinalDepth, op.StepDown), feeds, gen.PocketParams{
		ToolRadius:      tc.Tool.Diameter / 2,
		StepOver:        op.StepOver,
		Climb:           op.Climb,
		Islands:         op.Islands,
		FinishAllowance: op.FinishAllowance,
		Pattern:         op.Pattern,
		OneWay:          op.OneWay,
	})
	if err != nil {
		return gcode.Path{}, fmt.Errorf("pocket operation %q: %w", op.OpLabel, err)
	}
	return op.frame(cmds), nil
}
