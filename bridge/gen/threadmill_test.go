// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// TestThreadMillHelix checks the thread is cut as a descending helix of arcs at the thread
// radius, with one full turn (two 180° arcs) per pitch, spanning the thread top to bottom.
func TestThreadMillHelix(t *testing.T) {
	top := gcode.Vector3{X: 5, Y: 5, Z: 0}
	bottom := gcode.Vector3{X: 5, Y: 5, Z: -6}
	cmds, err := GenerateThreadMill(top, bottom, ThreadMillParams{MajorRadius: 4, ToolRadius: 1, Pitch: 2, Internal: true, Climb: true, RetractHeight: 10})
	if err != nil {
		t.Fatalf("GenerateThreadMill: %v", err)
	}
	// internal climb → CCW → G3 arcs.
	arcs := 0
	lowest, highest := math.Inf(1), math.Inf(-1)
	for _, c := range cmds {
		if c.Name == "G3" {
			arcs++
			if z, ok := c.Params["Z"]; ok {
				lowest, highest = math.Min(lowest, z), math.Max(highest, z)
			}
		}
		if c.Name == "G2" {
			t.Errorf("internal climb thread should be all G3, found a G2: %+v", c.Params)
		}
	}
	// 3 turns (6mm / 2mm pitch) → 6 half-arcs + 2 lead arcs = 8.
	if arcs != 8 {
		t.Errorf("arc count = %d, want 8 (3 turns ×2 + lead-in + lead-out)", arcs)
	}
	if !approx(lowest, -6) {
		t.Errorf("thread reaches z=%g, want the bottom -6", lowest)
	}
	if highest > 0 {
		t.Errorf("thread highest cut z=%g, should not rise above the top 0", highest)
	}
}

// TestThreadMillOrbitRadius checks the tool-centre orbit is the major radius offset by the tool
// radius — inward for internal, outward for external.
func TestThreadMillOrbitRadius(t *testing.T) {
	top := gcode.Vector3{X: 0, Y: 0, Z: 0}
	bottom := gcode.Vector3{X: 0, Y: 0, Z: -2}
	internal, _ := GenerateThreadMill(top, bottom, ThreadMillParams{MajorRadius: 4, ToolRadius: 1, Pitch: 2, Internal: true, Climb: true})
	if r := maxAbsX(internal); !approx(r, 3) {
		t.Errorf("internal orbit radius = %g, want 3 (major 4 − tool 1)", r)
	}
	external, _ := GenerateThreadMill(top, bottom, ThreadMillParams{MajorRadius: 4, ToolRadius: 1, Pitch: 2, Internal: false, Climb: true})
	if r := maxAbsX(external); !approx(r, 5) {
		t.Errorf("external orbit radius = %g, want 5 (major 4 + tool 1)", r)
	}
}

// maxAbsX returns the largest |X| any move reaches (the orbit radius about an axis at X=0).
func maxAbsX(cmds []gcode.Command) float64 {
	m := 0.0
	for _, c := range cmds {
		if x, ok := c.Params["X"]; ok {
			m = math.Max(m, math.Abs(x))
		}
	}
	return m
}

// TestThreadMillErrors covers the degenerate geometry and tool cases.
func TestThreadMillErrors(t *testing.T) {
	top := gcode.Vector3{X: 0, Y: 0, Z: 0}
	bottom := gcode.Vector3{X: 0, Y: 0, Z: -2}
	if _, err := GenerateThreadMill(top, bottom, ThreadMillParams{MajorRadius: 1, ToolRadius: 2, Pitch: 1, Internal: true}); err == nil {
		t.Error("a tool larger than the internal major radius must error")
	}
	if _, err := GenerateThreadMill(top, bottom, ThreadMillParams{MajorRadius: 4, ToolRadius: 1, Pitch: 0, Internal: true}); err == nil {
		t.Error("a non-positive pitch must error")
	}
	if _, err := GenerateThreadMill(top, gcode.Vector3{X: 1, Y: 0, Z: -2}, ThreadMillParams{MajorRadius: 4, Pitch: 1}); err == nil {
		t.Error("a non-vertical thread axis must error")
	}
}
