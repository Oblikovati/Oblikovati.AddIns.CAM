// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// surfFeeds is a feed set for surface-finish tests.
func surfFeeds() Feeds { return Feeds{Vert: 60, Horiz: 300, ClearanceZ: 20, SafeZ: 2} }

// twoRows are two scan-line rows of cutter-location points (mm).
func twoRows() [][]gcode.Vector3 {
	return [][]gcode.Vector3{
		{{X: 0, Y: 0, Z: 1}, {X: 0, Y: 5, Z: 2}, {X: 0, Y: 10, Z: 1}},
		{{X: 5, Y: 0, Z: 1}, {X: 5, Y: 5, Z: 3}, {X: 5, Y: 10, Z: 1}},
	}
}

// TestGenerateSurfaceFinishOneWay checks one-way passes each retract, rapid, plunge, then feed
// the row in its given direction.
func TestGenerateSurfaceFinishOneWay(t *testing.T) {
	cmds, err := GenerateSurfaceFinish(twoRows(), surfFeeds(), SurfaceFinishParams{ClearanceZ: 20, Zigzag: false})
	if err != nil {
		t.Fatalf("GenerateSurfaceFinish: %v", err)
	}
	if cmds[0].Name != "G0" || cmds[0].Params["Z"] != 20 {
		t.Errorf("first move = %+v, want a clearance retract", cmds[0])
	}
	// both rows run +Y: the first feed of each row goes to Y=5 then Y=10
	feeds := cutFeedYs(cmds)
	if len(feeds) < 4 || feeds[0] != 5 || feeds[1] != 10 || feeds[2] != 5 || feeds[3] != 10 {
		t.Errorf("one-way rows should both run +Y (5 then 10); got feed Ys %v", feeds)
	}
	last := cmds[len(cmds)-1]
	if last.Name != "G0" || last.Params["Z"] != 20 {
		t.Errorf("last move = %+v, want a final retract", last)
	}
}

// TestGenerateSurfaceFinishZigzag checks the second row is reversed (starts at Y=10).
func TestGenerateSurfaceFinishZigzag(t *testing.T) {
	cmds, err := GenerateSurfaceFinish(twoRows(), surfFeeds(), SurfaceFinishParams{ClearanceZ: 20, Zigzag: true})
	if err != nil {
		t.Fatalf("GenerateSurfaceFinish: %v", err)
	}
	feeds := cutFeedYs(cmds)
	// row0 +Y (5,10); row1 reversed −Y (5,0)
	if len(feeds) < 4 || feeds[0] != 5 || feeds[1] != 10 || feeds[2] != 5 || feeds[3] != 0 {
		t.Errorf("zigzag should reverse row 1 (…,5,0); got feed Ys %v", feeds)
	}
}

// TestGenerateSurfaceFinishEmpty errors when no row has enough points.
func TestGenerateSurfaceFinishEmpty(t *testing.T) {
	if _, err := GenerateSurfaceFinish([][]gcode.Vector3{{{X: 0}}}, surfFeeds(), SurfaceFinishParams{}); err == nil {
		t.Error("a grid with no usable rows must error")
	}
}

// cutFeedYs returns the Y of each G1 cutting move that carries X/Y (skips pure-Z plunges).
func cutFeedYs(cmds []gcode.Command) []float64 {
	var ys []float64
	for _, c := range cmds {
		if c.Name != "G1" {
			continue
		}
		if _, hasX := c.Params["X"]; !hasX {
			continue // pure-Z plunge
		}
		ys = append(ys, c.Params["Y"])
	}
	return ys
}
