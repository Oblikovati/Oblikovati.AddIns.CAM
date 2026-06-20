// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
	"oblikovati.org/cam/bridge/geom2d"
)

// SlotOp cuts a channel of a given Width centred on the boundary path, stepping down in Z. A
// width equal to the tool diameter is a single centreline pass; a wider slot adds symmetric side
// passes. Unlike a pocket, the cleared band is centred on the path — for O-ring grooves,
// channels, and troughs. The driving Boundary (a closed XY region in millimetres) is populated
// by the engine; Execute is pure given the boundary, tool, and depths.
type SlotOp struct {
	OpBase
	Width    float64        // mm — full slot width (>= tool diameter)
	StepOver float64        // side-pass spacing as a fraction of the tool diameter (0..1); 0 → 0.75
	Climb    bool           // climb vs conventional milling
	StepDown float64        // max Z step per pass (mm); <=0 → single pass
	Boundary geom2d.Polygon // centreline contour (mm), populated by the engine
}

// Features reports the property groups a slot uses.
func (op *SlotOp) Features() FeatureFlag {
	return FeatureTool | FeatureDepths | FeatureHeights | FeatureStepDown | FeatureBaseGeometry
}

// Execute generates the slot toolpath: resolve the tool radius and feeds, build the depth
// levels, and delegate to the slot generator, wrapped in the standard op framing.
func (op *SlotOp) Execute(job *Job) (gcode.Path, error) {
	tc, err := op.resolveTool(job)
	if err != nil {
		return gcode.Path{}, err
	}
	if len(op.Boundary) < 3 {
		return gcode.Path{}, fmt.Errorf("slot operation %q has no boundary contour", op.OpLabel)
	}
	feeds := gen.Feeds{Vert: tc.VertFeed, Horiz: tc.HorizFeed, ClearanceZ: op.ClearanceHeight, SafeZ: op.SafeHeight}
	cmds, err := gen.GenerateSlot(op.Boundary, gen.DepthLevels(op.StartDepth, op.FinalDepth, op.StepDown), feeds, gen.SlotParams{
		ToolRadius: tc.Tool.Diameter / 2,
		Width:      op.Width,
		StepOver:   op.StepOver,
		Climb:      op.Climb,
	})
	if err != nil {
		return gcode.Path{}, fmt.Errorf("slot operation %q: %w", op.OpLabel, err)
	}
	return op.frame(cmds), nil
}
