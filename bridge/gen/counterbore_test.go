// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// TestCounterboreClearsRecess checks the counterbore clears a flat-bottom recess with helical
// arcs reaching the recess bottom and staying within the recess radius.
func TestCounterboreClearsRecess(t *testing.T) {
	top := gcode.Vector3{X: 0, Y: 0, Z: 0}
	bottom := gcode.Vector3{X: 0, Y: 0, Z: -3}
	cmds, err := GenerateCounterbore(top, bottom, CounterboreParams{Diameter: 12, ToolDiameter: 4, Pitch: 1, StepOver: 0.5})
	if err != nil {
		t.Fatalf("GenerateCounterbore: %v", err)
	}
	arcs, lowest, maxR := 0, math.Inf(1), 0.0
	for _, c := range cmds {
		if c.Name == "G2" || c.Name == "G3" {
			arcs++
			if z, ok := c.Params["Z"]; ok {
				lowest = math.Min(lowest, z)
			}
			maxR = math.Max(maxR, math.Abs(c.Params["X"]))
		}
	}
	if arcs == 0 {
		t.Fatal("counterbore emitted no helical arcs")
	}
	if !approx(lowest, -3) {
		t.Errorf("recess reaches z=%g, want the bottom -3", lowest)
	}
	// the tool centre stays within the outer radius (diameter/2 − tool radius = 6−2 = 4).
	if maxR > 4+1e-6 {
		t.Errorf("tool centre orbit %g exceeds the recess outer radius 4", maxR)
	}
}

// TestCounterboreErrors covers the degenerate tool/diameter/pitch cases.
func TestCounterboreErrors(t *testing.T) {
	top := gcode.Vector3{X: 0, Y: 0, Z: 0}
	bottom := gcode.Vector3{X: 0, Y: 0, Z: -3}
	if _, err := GenerateCounterbore(top, bottom, CounterboreParams{Diameter: 12, ToolDiameter: 0, Pitch: 1}); err == nil {
		t.Error("a zero tool diameter must error")
	}
	if _, err := GenerateCounterbore(top, bottom, CounterboreParams{Diameter: 4, ToolDiameter: 4, Pitch: 1}); err == nil {
		t.Error("a recess no wider than the tool must error")
	}
	if _, err := GenerateCounterbore(top, bottom, CounterboreParams{Diameter: 12, ToolDiameter: 4, Pitch: 0}); err == nil {
		t.Error("a non-positive pitch must error")
	}
}
