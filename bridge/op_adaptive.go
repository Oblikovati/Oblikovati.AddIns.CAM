// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
	"oblikovati.org/cam/bridge/geom2d"
)

// AdaptiveOp clears the interior of a boundary region with HSM adaptive clearing — a continuous
// low-engagement inward spiral that stays down between rings, so the controller can hold a high
// feed. It is the high-speed analogue of PocketOp (which retracts between full-width rings). The
// driving Boundary (a closed XY region in millimetres) is populated by the engine; Execute is
// pure given the boundary, tool, and depths.
type AdaptiveOp struct {
	OpBase
	StepOver        float64          // radial engagement as a fraction of the tool diameter (0..1); 0 → 0.1
	Climb           bool             // climb vs conventional milling
	StepDown        float64          // max Z step per pass (mm); <=0 → single pass
	FinishAllowance float64          // mm of stock left on the walls when roughing; >0 adds a finish pass
	Boundary        geom2d.Polygon   // driving region (mm), populated by the engine
	Islands         []geom2d.Polygon // regions to leave standing (holes/bosses); the clearing routes around them
}

// Features reports the property groups an adaptive clearing op uses.
func (op *AdaptiveOp) Features() FeatureFlag {
	return FeatureTool | FeatureDepths | FeatureHeights | FeatureStepDown | FeatureBaseGeometry
}

// Execute generates the adaptive clearing toolpath: resolve the tool radius and feeds, build the
// depth levels, and delegate to the adaptive generator, wrapped in the standard op framing.
func (op *AdaptiveOp) Execute(job *Job) (gcode.Path, error) {
	tc, err := op.resolveTool(job)
	if err != nil {
		return gcode.Path{}, err
	}
	if len(op.Boundary) < 3 {
		return gcode.Path{}, fmt.Errorf("adaptive operation %q has no boundary region", op.OpLabel)
	}
	feeds := op.feeds(tc)
	cmds, err := gen.GenerateAdaptive(op.Boundary, gen.DepthLevels(op.StartDepth, op.FinalDepth, op.StepDown), feeds, gen.AdaptiveParams{
		ToolRadius:      tc.Tool.Diameter / 2,
		StepOver:        op.StepOver,
		Climb:           op.Climb,
		Islands:         op.Islands,
		FinishAllowance: op.FinishAllowance,
	})
	if err != nil {
		return gcode.Path{}, fmt.Errorf("adaptive operation %q: %w", op.OpLabel, err)
	}
	return op.frame(cmds), nil
}
