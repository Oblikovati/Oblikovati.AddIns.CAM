// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// TestChooseVoxelResResolvesTool checks the cell size resolves the smallest cutter (half its radius)
// when the stock is small enough to stay under the cell cap.
func TestChooseVoxelResResolvesTool(t *testing.T) {
	res := chooseVoxelRes(gcode.Vector3{}, gcode.Vector3{X: 20, Y: 20, Z: 20}, 3)
	if res != 1.5 {
		t.Errorf("res = %v, want 1.5 (half the 3 mm tool radius)", res)
	}
}

// TestChooseVoxelResCapsCellCount checks a large stock is coarsened so the grid stays under the cap.
func TestChooseVoxelResCapsCellCount(t *testing.T) {
	min, max := gcode.Vector3{}, gcode.Vector3{X: 1000, Y: 1000, Z: 1000}
	res := chooseVoxelRes(min, max, 1)
	if res <= 0.5 {
		t.Errorf("res = %v, expected coarsening above the 0.5 mm tool-driven size", res)
	}
	g := NewVoxelGrid(min, max, res)
	if g.Total() > voxelCellCap {
		t.Errorf("grid has %d cells, over the cap %d", g.Total(), voxelCellCap)
	}
}

// TestCutPointsChainsMoves checks the playback polyline is the first move's start followed by every
// move's end.
func TestCutPointsChainsMoves(t *testing.T) {
	cuts := []voxelMove{
		{from: gcode.Vector3{X: 0}, to: gcode.Vector3{X: 1}},
		{from: gcode.Vector3{X: 1}, to: gcode.Vector3{X: 2}},
	}
	pts := cutPoints(cuts)
	if len(pts) != 3 || pts[0].X != 0 || pts[2].X != 2 {
		t.Errorf("points = %+v, want x 0,1,2", pts)
	}
}

// TestMinCutterRadius checks the smallest cutter radius drives the resolution choice.
func TestMinCutterRadius(t *testing.T) {
	cuts := []voxelMove{
		{cutter: Cutter{Radius: 4}},
		{cutter: Cutter{Radius: 1.5}},
	}
	if r := minCutterRadius(cuts); r != 1.5 {
		t.Errorf("minCutterRadius = %v, want 1.5", r)
	}
}
