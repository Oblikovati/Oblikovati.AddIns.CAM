// SPDX-License-Identifier: GPL-2.0-only

package dressup

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// profileLoop builds a finely discretised square profile (40 mm perimeter sampled every
// 1 mm) at cut depth z — realistic geometry where tabs touch only a few short segments.
func profileLoop(z float64) gcode.Path {
	cmds := []gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"Z": 15}),
		gcode.NewCommand("G0", map[string]float64{"X": 0, "Y": 0}),
		gcode.NewCommand("G1", map[string]float64{"Z": z, "F": 50}), // plunge
	}
	// Square corners traversed in 1 mm steps.
	edges := [][2][2]float64{{{0, 0}, {10, 0}}, {{10, 0}, {10, 10}}, {{10, 10}, {0, 10}}, {{0, 10}, {0, 0}}}
	for _, e := range edges {
		for s := 1.0; s <= 10; s++ {
			x := e[0][0] + (e[1][0]-e[0][0])*s/10
			y := e[0][1] + (e[1][1]-e[0][1])*s/10
			cmds = append(cmds, gcode.NewCommand("G1", map[string]float64{"X": x, "Y": y, "F": 200}))
		}
	}
	return gcode.NewPath(append(cmds, gcode.NewCommand("G0", map[string]float64{"Z": 15})))
}

// TestApplyTagsLiftsSomeMoves: with two tabs, some cutting moves are raised above the cut
// depth and at least one stays at depth — and the original path is not mutated.
func TestApplyTagsLiftsSomeMoves(t *testing.T) {
	in := profileLoop(-5)
	out := ApplyTags(in, TagParams{Count: 2, Width: 4, Height: 3})

	lifted, atDepth := 0, 0
	for _, c := range out.Commands {
		if c.Name != "G1" {
			continue
		}
		if _, ok := c.Params["X"]; !ok {
			continue // the plunge
		}
		switch z := c.Params["Z"]; {
		case z > -5+1e-9:
			lifted++
		case z == -5:
			atDepth++
		}
	}
	if lifted == 0 {
		t.Error("expected some cutting moves raised over tabs")
	}
	if atDepth == 0 {
		t.Error("expected some cutting moves to stay at cut depth")
	}
	// Tab tops are exactly cut depth + height.
	for _, c := range out.Commands {
		if c.Name == "G1" {
			if _, ok := c.Params["X"]; ok && c.Params["Z"] > -5+1e-9 && c.Params["Z"] != -2 {
				t.Errorf("tab top Z = %g, want -2 (=-5+3)", c.Params["Z"])
			}
		}
	}
	// The input is untouched (the plunge move still carries no later mutation).
	if in.Commands[3].Params["Z"] != 0 && len(in.Commands[3].Params) != 3 {
		t.Error("ApplyTags must not mutate the input path")
	}
}

// TestApplyTagsNoop: zero count or width leaves the path unchanged.
func TestApplyTagsNoop(t *testing.T) {
	in := profileLoop(-5)
	if got := ApplyTags(in, TagParams{Count: 0, Width: 4, Height: 3}); len(got.Commands) != len(in.Commands) {
		t.Error("zero count should return the path unchanged")
	}
	out := ApplyTags(in, TagParams{Count: 2, Width: 0, Height: 3})
	for _, c := range out.Commands {
		if c.Name == "G1" {
			if _, ok := c.Params["X"]; ok && c.Params["Z"] != 0 {
				// no Z was added to cut moves
				t.Errorf("zero width should add no tab lift, got Z=%g", c.Params["Z"])
			}
		}
	}
}
