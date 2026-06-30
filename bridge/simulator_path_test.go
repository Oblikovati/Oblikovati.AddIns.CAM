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

// TestSegmentLines checks the selected segments become independent line pairs in host centimetres.
func TestSegmentLines(t *testing.T) {
	pts := toolpathFromGCode("G1 X0 Y0 Z0\nG1 X100\nG1 Y100") // 3 points, 2 segments
	coords, indices := segmentLines(pts, func(i int) bool { return i == 1 })
	if len(indices) != 2 || indices[0] != 0 || indices[1] != 1 {
		t.Errorf("indices = %v, want one segment [0 1]", indices)
	}
	if len(coords) != 6 { // one segment × 2 points × xyz
		t.Fatalf("coords = %d, want 6", len(coords))
	}
	if coords[0] != 10 { // segment 1 starts at X=100 mm → 10 cm
		t.Errorf("cm conversion wrong: %v", coords[0])
	}
}

// TestMotionWithKinds checks each motion point is tagged feed (cut) or rapid by its move.
func TestMotionWithKinds(t *testing.T) {
	pts, feed := motionWithKinds("G0 X0 Y0 Z5\nG1 Z0\nG1 X10\nG0 Z5")
	if len(pts) != 4 || len(feed) != 4 {
		t.Fatalf("points/feed = %d/%d, want 4/4", len(pts), len(feed))
	}
	want := []bool{false, true, true, false} // rapid, feed, feed, rapid
	for i := range want {
		if feed[i] != want[i] {
			t.Errorf("feed[%d] = %v, want %v", i, feed[i], want[i])
		}
	}
}
