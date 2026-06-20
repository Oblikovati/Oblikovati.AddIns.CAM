// SPDX-License-Identifier: GPL-2.0-only

package dressup

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// squareProfile is a CCW square cut loop shaped like the profile generator's walkLoop output:
// rapids in/out, a plunge, then four cutting moves closing back to the start.
func squareProfile() gcode.Path {
	g := func(name string, p map[string]float64) gcode.Command { return gcode.NewCommand(name, p) }
	return gcode.Path{Commands: []gcode.Command{
		g("G0", map[string]float64{"Z": 5}),
		g("G0", map[string]float64{"X": 0, "Y": 0}),
		g("G0", map[string]float64{"Z": 1}),
		g("G1", map[string]float64{"Z": -2, "F": 100}),
		g("G1", map[string]float64{"X": 10, "Y": 0, "F": 200}),
		g("G1", map[string]float64{"X": 10, "Y": 10, "F": 200}),
		g("G1", map[string]float64{"X": 0, "Y": 10, "F": 200}),
		g("G1", map[string]float64{"X": 0, "Y": 0, "F": 200}),
		g("G0", map[string]float64{"Z": 5}),
	}}
}

// TestApplyDogboneSquare inserts a bone at every corner of a CCW square (four left turns) and
// checks two bone moves were spliced in per corner, inheriting the cut feed.
func TestApplyDogboneSquare(t *testing.T) {
	in := squareProfile()
	out := ApplyDogbone(in, DogboneParams{Style: StyleTBoneH, Length: 1, MinAngle: pi / 4, Side: SideLeft})

	wantInserted := 4 * 2
	if got := len(out.Commands) - len(in.Commands); got != wantInserted {
		t.Fatalf("inserted %d commands, want %d", got, wantInserted)
	}
	bones := 0
	// every bone move must carry the loop feed so the spliced G-code stays valid
	for i, c := range out.Commands {
		if c.Name == "G1" {
			if _, hasZ := c.Params["Z"]; !hasZ {
				if c.Params["F"] != 200 {
					t.Errorf("cut/bone move %d missing feed: %+v", i, c.Params)
				}
				bones++
			}
		}
	}
	if bones != 4+wantInserted { // 4 original cuts + 8 bone moves
		t.Errorf("counted %d cut+bone moves, want %d", bones, 4+wantInserted)
	}
}

// TestApplyDogboneNoOp leaves the path untouched when length is zero or no corner qualifies.
func TestApplyDogboneNoOp(t *testing.T) {
	in := squareProfile()
	if out := ApplyDogbone(in, DogboneParams{Style: StyleDogbone, Length: 0}); len(out.Commands) != len(in.Commands) {
		t.Errorf("zero length must be a no-op")
	}
	// right-side bones on a CCW (left-turning) loop: nothing qualifies
	if out := ApplyDogbone(in, DogboneParams{Style: StyleDogbone, Length: 1, MinAngle: pi / 4, Side: SideRight}); len(out.Commands) != len(in.Commands) {
		t.Errorf("no right-side corners on a CCW loop must be a no-op")
	}
}
