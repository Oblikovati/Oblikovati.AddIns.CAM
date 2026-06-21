// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
)

// ThreadMillOp cuts a thread into each detected hole by helical interpolation — a single thread
// mill orbiting the hole axis, advancing one pitch per turn. It reuses the part's holes (like
// Drilling/Helix) but produces a thread instead of a plain bore, so it suits tapped holes cut
// without a tap.
type ThreadMillOp struct {
	OpBase
	MajorDiameter float64 // mm — nominal thread diameter
	Pitch         float64 // mm — thread lead (axial advance per turn)
	Internal      bool    // internal (in a hole) vs external (on a boss)
	Climb         bool    // climb vs conventional milling
	Holes         []DrillTarget
}

// Features reports the property groups thread milling uses.
func (op *ThreadMillOp) Features() FeatureFlag {
	return FeatureTool | FeatureDepths | FeatureHeights | FeatureBaseGeometry
}

// Execute generates the thread-milling toolpath for each hole: a helical thread of the major
// diameter cut from the hole top to its bottom, retracting to the clearance plane between holes.
func (op *ThreadMillOp) Execute(job *Job) (gcode.Path, error) {
	tc, err := op.resolveTool(job)
	if err != nil {
		return gcode.Path{}, err
	}
	if len(op.Holes) == 0 {
		return gcode.Path{}, fmt.Errorf("thread mill operation %q has no holes to thread", op.OpLabel)
	}
	var cutting []gcode.Command
	for _, h := range orderedHoles(op.Holes) {
		cmds, err := gen.GenerateThreadMill(
			gcode.Vector3{X: h.X, Y: h.Y, Z: h.Top}, gcode.Vector3{X: h.X, Y: h.Y, Z: h.Bottom},
			gen.ThreadMillParams{
				MajorRadius: op.MajorDiameter / 2, ToolRadius: tc.Tool.Diameter / 2,
				Pitch: op.Pitch, Internal: op.Internal, Climb: op.Climb, RetractHeight: op.ClearanceHeight,
			})
		if err != nil {
			return gcode.Path{}, fmt.Errorf("thread mill operation %q, hole at (%g,%g): %w", op.OpLabel, h.X, h.Y, err)
		}
		cutting = append(cutting, cmds...)
	}
	return op.frame(cutting), nil
}
