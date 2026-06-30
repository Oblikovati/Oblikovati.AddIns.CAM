// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// drillingJob is a block with two through-holes drilled by a 6 mm drill — the canned-cycle case the
// simulator previously could not carve.
func drillingJob() *Job {
	j := NewJob()
	j.Stock = Stock{Min: gcode.Vector3{Z: -12}, Max: gcode.Vector3{X: 40, Y: 40, Z: 0}}
	j.Tools = []ToolController{{Tool: ToolBit{ShapeType: "drill", Diameter: 6}, VertFeed: 100, HorizFeed: 300}}
	j.Operations = []Operation{&DrillingOp{
		OpBase: OpBase{OpLabel: "Drill", IsActive: true, ClearanceHeight: 10, RetractHeight: 2, StartDepth: 0, FinalDepth: -12},
		Holes:  []DrillTarget{{X: 10, Y: 10, Top: 0, Bottom: -12}, {X: 30, Y: 30, Top: 0, Bottom: -12}},
	}}
	return j
}

// TestDrillingCutsReachHoleBottoms checks flattenCuts (via canned-cycle expansion) produces plunge
// moves reaching each hole bottom with the drill's cone cutter.
func TestDrillingCutsReachHoleBottoms(t *testing.T) {
	results, err := drillingJob().GenerateAll()
	if err != nil {
		t.Fatalf("GenerateAll: %v", err)
	}
	plunges := 0
	for _, c := range flattenCuts(results, 12) {
		if c.to.Z <= -11.9 && c.from.Z > c.to.Z && c.cutter.Shape == CutterCone {
			plunges++
		}
	}
	if plunges < 2 {
		t.Errorf("plunge cuts to hole bottoms = %d, want >= 2", plunges)
	}
}

// TestDrilledHoleIsCarved checks the voxel grid actually has material removed at a hole centre after
// the full carve — the end-to-end fix for the canned-cycle limitation.
func TestDrilledHoleIsCarved(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.lastJob = drillingJob()
	if _, err := e.simulateAction(); err != nil {
		t.Fatalf("simulateAction: %v", err)
	}
	for i := 0; i < len(e.simPath); i++ {
		_, _ = e.simStepAction()
	}
	if i, j, k, ok := e.voxel.cellAt(gcode.Vector3{X: 10, Y: 10, Z: -6}); !ok || e.voxel.Occupied(i, j, k) {
		t.Errorf("hole centre still solid after drilling (ok=%v)", ok)
	}
}
