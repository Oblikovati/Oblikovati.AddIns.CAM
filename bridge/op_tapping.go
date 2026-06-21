// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
)

// TappingOp cuts internal threads in a set of pre-drilled holes with a tapping canned cycle. It
// reuses the part's cylindrical holes (like Drilling), but instead of a plain drill cycle it
// feeds a tap in synchronised with the spindle at the thread Pitch and reverses out — emitting a
// G84 (right-hand) or G74 (left-hand) cycle per hole. The cycle runs in feed-per-revolution (G95)
// mode, so its F is the pitch advanced once per spindle turn — independent of rpm — matching the
// reference workbench. A spindle speed is still required (it synchronises the cycle and lets a post
// that expands the cycle recover the per-minute feed).
type TappingOp struct {
	OpBase
	Pitch     float64 // mm per thread (and per spindle revolution)
	LeftHand  bool    // cut a left-hand thread (G74) instead of a right-hand one (G84)
	DwellTime float64 // optional dwell at the bottom of each hole (s)
	Holes     []DrillTarget
}

// Features reports the property groups tapping uses (a tool, depths, heights, and the
// cylindrical-face geometry it is driven by — no step-down: the cycle taps full depth at once).
func (op *TappingOp) Features() FeatureFlag {
	return FeatureTool | FeatureDepths | FeatureHeights | FeatureBaseGeometry
}

// Execute generates the tapping toolpath: switch to feed-per-revolution (G95) so each cycle's F is
// the thread pitch advanced once per spindle turn, rapid to the clearance plane, then for each hole
// rapid over its XY and emit the synchronised tap cycle, cancel the canned cycle with G80, and
// restore feed-per-minute (G94). Returns an error if the tool controller is missing, the spindle is
// stopped (no rpm to synchronise against), or any hole's geometry/pitch is illegal.
func (op *TappingOp) Execute(job *Job) (gcode.Path, error) {
	tc, err := op.resolveTool(job)
	if err != nil {
		return gcode.Path{}, err
	}
	if len(op.Holes) == 0 {
		return gcode.Path{}, fmt.Errorf("tapping operation %q has no holes to tap", op.OpLabel)
	}
	if tc.SpindleSpeed <= 0 {
		return gcode.Path{}, fmt.Errorf("tapping operation %q needs a running spindle to synchronise the feed, got %g rpm", op.OpLabel, tc.SpindleSpeed)
	}

	cutting := []gcode.Command{
		gcode.NewCommand("G95", nil), // feed per revolution: F is the pitch, advanced once per turn
		gcode.NewCommand("G0", map[string]float64{"Z": op.ClearanceHeight}),
	}
	for _, h := range orderedHoles(op.Holes) {
		cmds, err := gen.GenerateTap(
			gcode.Vector3{X: h.X, Y: h.Y, Z: h.Top},
			gcode.Vector3{X: h.X, Y: h.Y, Z: h.Bottom},
			tc.SpindleSpeed,
			gen.TapParams{Pitch: op.Pitch, LeftHand: op.LeftHand, DwellTime: op.DwellTime},
		)
		if err != nil {
			return gcode.Path{}, fmt.Errorf("tapping operation %q, hole at (%g,%g): %w", op.OpLabel, h.X, h.Y, err)
		}
		cutting = append(cutting, gcode.NewCommand("G0", map[string]float64{"X": h.X, "Y": h.Y}))
		cutting = append(cutting, cmds...)
	}
	cutting = append(cutting,
		gcode.NewCommand("G80", nil), // cancel canned cycle
		gcode.NewCommand("G94", nil), // restore feed per minute
	)

	return op.frame(cutting), nil
}
