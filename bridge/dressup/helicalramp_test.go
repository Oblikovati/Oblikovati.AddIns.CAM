// SPDX-License-Identifier: GPL-2.0-only

package dressup

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// TestApplyHelicalRampReplacesPlunge replaces the straight plunge with a helical descent that
// orbits a circle of the given radius and ends back at the plunge point at depth.
func TestApplyHelicalRampReplacesPlunge(t *testing.T) {
	out := ApplyHelicalRamp(plungeLoop(), HelicalRampParams{Radius: 3, Pitch: 1})

	for _, c := range out.Commands {
		if isPlunge(c, 0, 0, 5) {
			t.Errorf("a straight plunge survived: %+v", c.Params)
		}
	}

	lo, hi := math.Inf(1), math.Inf(-1)
	maxR := 0.0
	inCut := false
	var lastBeforeCut gcode.Command
	for i, c := range out.Commands {
		if c.Name != "G1" {
			continue
		}
		if x, ok := c.Params["X"]; ok && x == 10 && lastBeforeCut.Name == "" {
			lastBeforeCut = out.Commands[i-1] // the move just before the first contour cut
			inCut = true
		}
		if z, ok := c.Params["Z"]; ok && !inCut {
			lo, hi = math.Min(lo, z), math.Max(hi, z)
		}
		// the helix orbits a circle tangent at (0,0); every helix point is within 2·radius of it.
		if x, okx := c.Params["X"]; okx && !inCut {
			if y, oky := c.Params["Y"]; oky {
				maxR = math.Max(maxR, math.Hypot(x, y))
			}
		}
	}
	if math.Abs(lo-(-2)) > 1e-9 {
		t.Errorf("helix lowest Z = %g, want -2 (the cut depth)", lo)
	}
	if hi >= 5 {
		t.Errorf("helix highest Z = %g, should be below the start (5)", hi)
	}
	if maxR > 2*3+1e-6 {
		t.Errorf("helix strayed %g from the plunge, beyond the 2·radius (6) entry circle", maxR)
	}
	if math.Abs(lastBeforeCut.Params["X"]) > 1e-9 || math.Abs(lastBeforeCut.Params["Y"]) > 1e-9 || lastBeforeCut.Params["Z"] != -2 {
		t.Errorf("helix should end at the plunge point (0,0,-2), got %+v", lastBeforeCut.Params)
	}
}

// TestApplyHelicalRampTurns checks a smaller pitch makes more turns (more descent moves).
func TestApplyHelicalRampTurns(t *testing.T) {
	coarse := ApplyHelicalRamp(plungeLoop(), HelicalRampParams{Radius: 3, Pitch: 3.5})
	fine := ApplyHelicalRamp(plungeLoop(), HelicalRampParams{Radius: 3, Pitch: 1})
	if len(fine.Commands) <= len(coarse.Commands) {
		t.Errorf("a smaller pitch should add helix turns: fine=%d coarse=%d", len(fine.Commands), len(coarse.Commands))
	}
}

// TestApplyHelicalRampNoOp leaves the path unchanged for zero params or a plunge with no cut.
func TestApplyHelicalRampNoOp(t *testing.T) {
	in := plungeLoop()
	if out := ApplyHelicalRamp(in, HelicalRampParams{Radius: 0, Pitch: 1}); len(out.Commands) != len(in.Commands) {
		t.Error("zero radius must be a no-op")
	}
	bare := gcode.Path{Commands: []gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"X": 0, "Y": 0, "Z": 5}),
		gcode.NewCommand("G1", map[string]float64{"Z": -2}),
	}}
	if out := ApplyHelicalRamp(bare, HelicalRampParams{Radius: 3, Pitch: 1}); len(out.Commands) != len(bare.Commands) {
		t.Error("a plunge with no following cut must stay a straight plunge")
	}
}

// TestApplyHelicalRampCapsTurns keeps a pathological pitch from exploding the path.
func TestApplyHelicalRampCapsTurns(t *testing.T) {
	out := ApplyHelicalRamp(plungeLoop(), HelicalRampParams{Radius: 3, Pitch: 1e-6})
	if len(out.Commands) > maxHelixTurns*helixSegmentsPerTurn+10 {
		t.Errorf("helix turns not capped: %d commands", len(out.Commands))
	}
}
