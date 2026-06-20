// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
	"oblikovati.org/cam/bridge/geom2d"
)

// RestOp clears only the wall/corner stock a previous, larger tool left behind — the band of the
// region a tool of PrevToolDiameter could not reach. It runs the same ring clearing as PocketOp
// but restricted to that band, so a small finishing tool cleans up the walls and corners without
// re-cutting the already-cleared interior. The driving Boundary (a closed XY region in
// millimetres) is populated by the engine; Execute is pure given the boundary, tool, and depths.
type RestOp struct {
	OpBase
	PrevToolDiameter float64          // diameter of the previous (larger) tool, mm
	StepOver         float64          // ring step as a fraction of the tool diameter (0..1); 0 → 0.5
	Climb            bool             // climb vs conventional milling
	StepDown         float64          // max Z step per pass (mm); <=0 → single pass
	Boundary         geom2d.Polygon   // driving region (mm), populated by the engine
	Islands          []geom2d.Polygon // standing regions; their walls leave their own band to clear
}

// Features reports the property groups a rest-machining op uses.
func (op *RestOp) Features() FeatureFlag {
	return FeatureTool | FeatureDepths | FeatureHeights | FeatureStepDown | FeatureBaseGeometry
}

// Execute generates the rest-clearing toolpath: resolve the tool radius and feeds, build the
// depth levels, and delegate to the rest generator (passing the previous tool's radius), wrapped
// in the standard op framing.
func (op *RestOp) Execute(job *Job) (gcode.Path, error) {
	tc, err := op.resolveTool(job)
	if err != nil {
		return gcode.Path{}, err
	}
	if len(op.Boundary) < 3 {
		return gcode.Path{}, fmt.Errorf("rest operation %q has no boundary region", op.OpLabel)
	}
	feeds := gen.Feeds{Vert: tc.VertFeed, Horiz: tc.HorizFeed, ClearanceZ: op.ClearanceHeight, SafeZ: op.SafeHeight}
	cmds, err := gen.GenerateRest(op.Boundary, gen.DepthLevels(op.StartDepth, op.FinalDepth, op.StepDown), feeds, gen.RestParams{
		ToolRadius: tc.Tool.Diameter / 2,
		PrevRadius: op.PrevToolDiameter / 2,
		StepOver:   op.StepOver,
		Climb:      op.Climb,
		Islands:    op.Islands,
	})
	if err != nil {
		return gcode.Path{}, fmt.Errorf("rest operation %q: %w", op.OpLabel, err)
	}
	return op.frame(cmds), nil
}
