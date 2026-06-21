// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/api/wire"
	"oblikovati.org/cam/bridge/gcode"
)

// retractResult is a one-operation result whose toolpath has a full between-cut retract to the
// clearance plane (Z=15), used to check keep-tool-down lowering. Cuts at Z=0, safe plane Z=2.
func retractResult() OperationResult {
	g0 := func(p map[string]float64) gcode.Command { return gcode.NewCommand("G0", p) }
	g1 := func(p map[string]float64) gcode.Command { return gcode.NewCommand("G1", p) }
	return OperationResult{
		Label:      "Pocket",
		SafeZ:      2,
		ClearanceZ: 15,
		Controller: ToolController{Tool: ToolBit{Diameter: 6}},
		Path: gcode.NewPath([]gcode.Command{
			g1(map[string]float64{"Z": 0, "F": 100}), g1(map[string]float64{"X": 10, "Y": 0, "F": 200}), // cut 1
			g0(map[string]float64{"Z": 15}), g0(map[string]float64{"X": 20, "Y": 0}), g0(map[string]float64{"Z": 2}), // retract
			g1(map[string]float64{"Z": 0, "F": 100}), g1(map[string]float64{"X": 30, "Y": 0, "F": 200}), // cut 2
			g0(map[string]float64{"Z": 15}), // final retract
		}),
	}
}

func clearanceLifts(p gcode.Path, z float64) int {
	n := 0
	for _, c := range p.Commands {
		if v, ok := c.Params["Z"]; ok && v == z {
			n++
		}
	}
	return n
}

// TestApplyKeepToolDownLowersRetractWhenClear: with the host reporting ample clearance, the
// between-cut bounce to the clearance plane is lowered to the safe plane, and the part body is
// actually queried (the mm↔cm projection runs).
func TestApplyKeepToolDownLowersRetractWhenClear(t *testing.T) {
	h := &recordingHost{minDistanceCm: 50} // 500 mm — every traverse clears
	e := NewEngine(h)
	job := NewJob()
	job.ModelBodies = []int{0}

	before := clearanceLifts(retractResult().Path, 15)
	out := e.applyKeepToolDown(job, []OperationResult{retractResult()})
	after := clearanceLifts(out[0].Path, 15)

	if after >= before {
		t.Errorf("clearance lifts %d → %d, want fewer after keep-tool-down", before, after)
	}
	if !h.called(wire.MethodBodyMinimumDistance) {
		t.Error("keep-tool-down must query the part via body.minimumDistance")
	}
}

// TestApplyKeepToolDownKeepsRetractWhenBlocked: the default host reports zero clearance (a wall in
// the way), so the always-safe full retract is preserved.
func TestApplyKeepToolDownKeepsRetractWhenBlocked(t *testing.T) {
	e := NewEngine(&recordingHost{}) // minDistanceCm 0 → blocked
	job := NewJob()
	job.ModelBodies = []int{0}

	before := clearanceLifts(retractResult().Path, 15)
	out := e.applyKeepToolDown(job, []OperationResult{retractResult()})
	if after := clearanceLifts(out[0].Path, 15); after != before {
		t.Errorf("blocked link changed the retract (%d → %d), want it kept", before, after)
	}
}

// TestApplyKeepToolDownNoBodyIsNoop: a job with no model body has nothing to collide with, so the
// paths are returned unchanged and the host is not queried.
func TestApplyKeepToolDownNoBodyIsNoop(t *testing.T) {
	h := &recordingHost{minDistanceCm: 50}
	e := NewEngine(h)
	out := e.applyKeepToolDown(NewJob(), []OperationResult{retractResult()})
	if clearanceLifts(out[0].Path, 15) != clearanceLifts(retractResult().Path, 15) {
		t.Error("a body-less job must not alter the toolpath")
	}
	if h.called(wire.MethodBodyMinimumDistance) {
		t.Error("a body-less job must not query the host")
	}
}
