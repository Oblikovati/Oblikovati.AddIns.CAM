// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestToolpathFromGCode checks the motion polyline is recovered with sticky axes (one point per
// move, non-motion lines ignored).
func TestToolpathFromGCode(t *testing.T) {
	program := "G0 X0 Y0 Z5\nM3 S1000\nG1 Z0 F100\nG1 X10\nG1 Y10\nG0 Z5"
	pts := toolpathFromGCode(program)
	if len(pts) != 5 { // the M3 line is not a move
		t.Fatalf("points = %d, want 5: %+v", len(pts), pts)
	}
	if pts[1].Z != 0 || pts[2].X != 10 || pts[3].Y != 10 || pts[4].Z != 5 {
		t.Errorf("polyline = %+v", pts)
	}
}

// TestPolylineLines checks the indexed line strip (cm) pairs consecutive points.
func TestPolylineLines(t *testing.T) {
	coords, indices := polylineLines(toolpathFromGCode("G1 X0 Y0 Z0\nG1 X100\nG1 Y100"))
	if len(coords) != 9 { // 3 points × xyz
		t.Errorf("coords = %d, want 9", len(coords))
	}
	if len(indices) != 4 || indices[0] != 0 || indices[1] != 1 || indices[2] != 1 || indices[3] != 2 {
		t.Errorf("indices = %v", indices)
	}
	if coords[3] != 10 { // X=100 mm → 10 cm
		t.Errorf("cm conversion wrong: %v", coords[3])
	}
}
