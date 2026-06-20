// SPDX-License-Identifier: GPL-2.0-only

package dressup

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// plungeLoop is a profile-style loop: rapids, a straight plunge from z=5 to z=-2, then a cut
// loop running +X first.
func plungeLoop() gcode.Path {
	g := func(name string, p map[string]float64) gcode.Command { return gcode.NewCommand(name, p) }
	return gcode.Path{Commands: []gcode.Command{
		g("G0", map[string]float64{"Z": 5}),
		g("G0", map[string]float64{"X": 0, "Y": 0}),
		g("G1", map[string]float64{"Z": -2, "F": 100}), // straight plunge, depth 7
		g("G1", map[string]float64{"X": 10, "Y": 0, "F": 200}),
		g("G1", map[string]float64{"X": 10, "Y": 10, "F": 200}),
	}}
}

// TestApplyRampReplacesPlunge replaces the straight plunge with a ramped descent that ends at
// the plunge point at depth.
func TestApplyRampReplacesPlunge(t *testing.T) {
	out := ApplyRamp(plungeLoop(), RampParams{Length: 3, Angle: math.Pi / 12}) // 15°

	// no pure-Z plunge should remain
	for _, c := range out.Commands {
		if isPlunge(c, 0, 0, 5) {
			t.Errorf("a straight plunge survived: %+v", c.Params)
		}
	}
	// the ramp's lowest move must reach the cut depth, the highest must be below the start
	lo, hi := math.Inf(1), math.Inf(-1)
	var lastBeforeCut gcode.Command
	for i, c := range out.Commands {
		if c.Name == "G1" {
			if z, ok := c.Params["Z"]; ok {
				lo, hi = math.Min(lo, z), math.Max(hi, z)
			}
		}
		if x, ok := c.Params["X"]; ok && x == 10 && lastBeforeCut.Name == "" {
			lastBeforeCut = out.Commands[i-1] // the move just before the first contour cut
		}
	}
	if math.Abs(lo-(-2)) > 1e-9 {
		t.Errorf("ramp lowest Z = %g, want -2 (the cut depth)", lo)
	}
	if hi >= 5 {
		t.Errorf("ramp highest Z = %g, should be below the start (5)", hi)
	}
	// the move handing off to the contour cut must be back at the plunge point (0,0,-2)
	if lastBeforeCut.Params["X"] != 0 || lastBeforeCut.Params["Y"] != 0 || lastBeforeCut.Params["Z"] != -2 {
		t.Errorf("ramp should end at the plunge point (0,0,-2), got %+v", lastBeforeCut.Params)
	}
}

// TestApplyRampNoOp leaves the path unchanged for zero params or a plunge with no following cut.
func TestApplyRampNoOp(t *testing.T) {
	in := plungeLoop()
	if out := ApplyRamp(in, RampParams{Length: 0, Angle: 1}); len(out.Commands) != len(in.Commands) {
		t.Error("zero length must be a no-op")
	}
	// a bare plunge with no subsequent cut stays a straight plunge
	bare := gcode.Path{Commands: []gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"X": 0, "Y": 0, "Z": 5}),
		gcode.NewCommand("G1", map[string]float64{"Z": -2}),
	}}
	if out := ApplyRamp(bare, RampParams{Length: 3, Angle: 0.3}); len(out.Commands) != len(bare.Commands) {
		t.Error("a plunge with no following cut must stay a straight plunge")
	}
}

// TestApplyRampCapsPasses keeps a near-flat angle from exploding the path.
func TestApplyRampCapsPasses(t *testing.T) {
	out := ApplyRamp(plungeLoop(), RampParams{Length: 0.01, Angle: 0.001}) // pathological
	if len(out.Commands) > maxRampPasses+10 {
		t.Errorf("ramp passes not capped: %d commands", len(out.Commands))
	}
}
