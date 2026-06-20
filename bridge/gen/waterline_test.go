// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// TestGenerateWaterline emits a plunge and a closed pass per loop at each level.
func TestGenerateWaterline(t *testing.T) {
	levels := []LevelLoops{
		{Z: 5, Loops: [][]gcode.Vector3{{{X: 0, Y: 0}, {X: 4, Y: 0}, {X: 4, Y: 4}, {X: 0, Y: 4}}}},
		{Z: 2, Loops: [][]gcode.Vector3{{{X: -1, Y: -1}, {X: 5, Y: -1}, {X: 5, Y: 5}, {X: -1, Y: 5}}}},
	}
	cmds, err := GenerateWaterline(levels, surfFeeds(), WaterlineParams{ClearanceZ: 20})
	if err != nil {
		t.Fatalf("GenerateWaterline: %v", err)
	}
	plunges := 0
	for _, c := range cmds {
		if _, hasZ := c.Params["Z"]; c.Name == "G1" && hasZ {
			plunges++
		}
	}
	if plunges != 2 { // one plunge per loop / level
		t.Errorf("want 2 plunges (one per level loop), got %d", plunges)
	}
	// each loop must be closed: last cutting move of a loop returns to its start
	if !closesLoop(cmds, 0, 0) {
		t.Error("a waterline loop must close back to its start")
	}
}

// TestGenerateWaterlineEmpty errors when nothing intersects.
func TestGenerateWaterlineEmpty(t *testing.T) {
	if _, err := GenerateWaterline(nil, surfFeeds(), WaterlineParams{}); err == nil {
		t.Error("no levels must error")
	}
	if _, err := GenerateWaterline([]LevelLoops{{Z: 1, Loops: [][]gcode.Vector3{{{X: 0}}}}}, surfFeeds(), WaterlineParams{}); err == nil {
		t.Error("only degenerate loops must error")
	}
}

// closesLoop reports whether some cutting move returns to (x, y).
func closesLoop(cmds []gcode.Command, x, y float64) bool {
	for _, c := range cmds {
		if c.Name == "G1" && c.Params["X"] == x && c.Params["Y"] == y {
			if _, hasZ := c.Params["Z"]; !hasZ {
				return true
			}
		}
	}
	return false
}
