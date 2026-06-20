// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// TestCountersinkCone checks the spiral starts at the rim on the surface and descends to the
// centre at the angle-derived depth, staying within the rim radius.
func TestCountersinkCone(t *testing.T) {
	center := gcode.Vector3{X: 10, Y: 10, Z: 0}
	cmds, err := GenerateCountersink(center, 200, CountersinkParams{Diameter: 8, ToolAngleDeg: 90, ToolDiameter: 4, StepOver: 0.5})
	if err != nil {
		t.Fatalf("GenerateCountersink: %v", err)
	}
	// the first feed move plunges to the surface (z=0) at the rim (x = 10 + 4).
	rimMove := cmds[0]
	if rimMove.Params["X"] != 14 {
		t.Errorf("spiral should start at the rim x=14, got %+v", rimMove.Params)
	}
	lowest, maxR := math.Inf(1), 0.0
	for _, c := range cmds {
		if z, ok := c.Params["Z"]; ok {
			lowest = math.Min(lowest, z)
		}
		if x, ok := c.Params["X"]; ok {
			maxR = math.Max(maxR, math.Abs(x-center.X))
		}
	}
	// 90° tool, rim radius 4 → cone depth = 4 / tan(45°) = 4.
	if !approx(lowest, -4) {
		t.Errorf("cone reaches z=%g, want -4 (rim radius / tan(45°))", lowest)
	}
	if maxR > 4+1e-9 {
		t.Errorf("spiral radius %g exceeds the rim radius 4", maxR)
	}
}

// TestCountersinkSpiralsInward checks the spiral ends near the centre (radius collapses).
func TestCountersinkSpiralsInward(t *testing.T) {
	center := gcode.Vector3{X: 0, Y: 0, Z: 0}
	cmds, err := GenerateCountersink(center, 200, CountersinkParams{Diameter: 10, ToolDiameter: 4})
	if err != nil {
		t.Fatalf("GenerateCountersink: %v", err)
	}
	last := cmds[len(cmds)-1]
	if r := math.Hypot(last.Params["X"], last.Params["Y"]); r > 0.3 {
		t.Errorf("spiral should end at the centre, ended at radius %g", r)
	}
}

// TestCountersinkErrors covers the degenerate tool/diameter cases.
func TestCountersinkErrors(t *testing.T) {
	center := gcode.Vector3{X: 0, Y: 0, Z: 0}
	if _, err := GenerateCountersink(center, 200, CountersinkParams{Diameter: 8, ToolDiameter: 0}); err == nil {
		t.Error("a zero tool diameter must error")
	}
	if _, err := GenerateCountersink(center, 200, CountersinkParams{Diameter: 0, ToolDiameter: 4}); err == nil {
		t.Error("a zero diameter must error")
	}
}
