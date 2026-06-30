// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// TestStampClearsUnderCutter checks a flat cutter clears the cells under its footprint and leaves
// cells outside its radius intact.
func TestStampClearsUnderCutter(t *testing.T) {
	g := NewVoxelGrid(gcode.Vector3{}, gcode.Vector3{X: 10, Y: 10, Z: 10}, 1)
	stampCutter(g, Cutter{Shape: CutterFlat, Radius: 2, Height: 10}, gcode.Vector3{X: 5, Y: 5, Z: 0})
	if g.Occupied(5, 5, 3) {
		t.Error("cell under the cutter centre not cleared")
	}
	if !g.Occupied(5, 9, 3) { // ~4 mm away in y, outside R=2
		t.Error("cell well outside the cutter was cleared")
	}
}

// TestStampRespectsHeight checks a short cutter only clears cells within its height above the tip.
func TestStampRespectsHeight(t *testing.T) {
	g := NewVoxelGrid(gcode.Vector3{}, gcode.Vector3{X: 10, Y: 10, Z: 10}, 1)
	stampCutter(g, Cutter{Shape: CutterFlat, Radius: 3, Height: 2}, gcode.Vector3{X: 5, Y: 5, Z: 5})
	if !g.Occupied(5, 5, 2) { // below the tip (z≈2.5 < 5) — untouched
		t.Error("cell below the tip was cleared")
	}
	if g.Occupied(5, 5, 5) { // just above the tip — within height
		t.Error("cell within the cutter height not cleared")
	}
}

// TestSweepSegmentCutsChannel checks sweeping a segment clears a continuous channel between its ends
// (densification leaves no gaps).
func TestSweepSegmentCutsChannel(t *testing.T) {
	g := NewVoxelGrid(gcode.Vector3{}, gcode.Vector3{X: 20, Y: 10, Z: 10}, 1)
	before := g.Count()
	sweepSegment(g, Cutter{Shape: CutterFlat, Radius: 1, Height: 10}, gcode.Vector3{X: 2, Y: 5, Z: 0}, gcode.Vector3{X: 18, Y: 5, Z: 0})
	for x := 3; x <= 17; x++ {
		if g.Occupied(x, 5, 5) {
			t.Fatalf("gap in channel at x=%d", x)
		}
	}
	if g.Count() >= before {
		t.Error("sweep removed nothing")
	}
}

// TestFlattenCutsCarriesToolPerOperation checks each operation's moves become cuts carrying that
// operation's cutter, with sticky positions across commands.
func TestFlattenCutsCarriesToolPerOperation(t *testing.T) {
	path := gcode.NewPath([]gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"X": 0, "Y": 0, "Z": 5}),
		gcode.NewCommand("G1", map[string]float64{"Z": 0}),
		gcode.NewCommand("G1", map[string]float64{"X": 10}),
	})
	res := []OperationResult{{
		Path:       path,
		Controller: ToolController{Tool: ToolBit{ShapeType: "ballend", Diameter: 4, CuttingEdgeHeight: 8}},
	}}
	cuts := flattenCuts(res, 30)
	if len(cuts) != 2 { // 3 motion points → 2 segments
		t.Fatalf("cuts = %d, want 2", len(cuts))
	}
	if cuts[0].cutter.Shape != CutterBall || cuts[0].cutter.Radius != 2 {
		t.Errorf("cut cutter = %+v, want ball r=2", cuts[0].cutter)
	}
	if cuts[1].to.X != 10 || cuts[1].from.Z != 0 {
		t.Errorf("second cut endpoints = %+v→%+v", cuts[1].from, cuts[1].to)
	}
}
