// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// cornerProbe is a three-point corner cycle: a Z touch-off and two edge probes.
func cornerProbe() []ProbePoint {
	return []ProbePoint{
		{Approach: gcode.Vector3{X: 5, Y: 5, Z: 2}, Target: gcode.Vector3{X: 5, Y: 5, Z: -5}},   // probe down
		{Approach: gcode.Vector3{X: -5, Y: 5, Z: -2}, Target: gcode.Vector3{X: 3, Y: 5, Z: -2}}, // probe +X
		{Approach: gcode.Vector3{X: 5, Y: -5, Z: -2}, Target: gcode.Vector3{X: 5, Y: 3, Z: -2}}, // probe +Y
	}
}

// TestProbeEmitsG38 checks each point emits a single G38.2 probe move addressing only the moved
// axis, at the probe feed.
func TestProbeEmitsG38(t *testing.T) {
	cmds, err := GenerateProbe(cornerProbe(), ProbeParams{ClearanceZ: 15, ProbeFeed: 50})
	if err != nil {
		t.Fatalf("GenerateProbe: %v", err)
	}
	var probes []gcode.Command
	for _, c := range cmds {
		if c.Name == "G38.2" {
			probes = append(probes, c)
		}
	}
	if len(probes) != 3 {
		t.Fatalf("got %d G38.2 moves, want 3", len(probes))
	}
	// the first probe is a pure -Z move: a Z target, no X/Y, at the probe feed.
	z := probes[0]
	if z.Params["Z"] != -5 || z.Params["F"] != 50 {
		t.Errorf("Z probe = %+v, want Z-5 F50", z.Params)
	}
	if _, hasX := z.Params["X"]; hasX {
		t.Errorf("Z probe should not move X/Y, got %+v", z.Params)
	}
	// the second probe is a pure +X move.
	if _, hasZ := probes[1].Params["Z"]; hasZ || probes[1].Params["X"] != 3 {
		t.Errorf("X probe = %+v, want X3 only", probes[1].Params)
	}
}

// TestProbeSetsWorkOffset checks each touch that names an axis emits a G10 L20 to zero that axis
// in the chosen work coordinate system.
func TestProbeSetsWorkOffset(t *testing.T) {
	pts := []ProbePoint{
		{Approach: gcode.Vector3{X: 5, Y: 5, Z: 2}, Target: gcode.Vector3{X: 5, Y: 5, Z: -5}, SetAxis: "Z"},
		{Approach: gcode.Vector3{X: -5, Y: 5, Z: -2}, Target: gcode.Vector3{X: 3, Y: 5, Z: -2}, SetAxis: "X"},
	}
	cmds, err := GenerateProbe(pts, ProbeParams{ClearanceZ: 15, ProbeFeed: 50, WorkOffset: 2})
	if err != nil {
		t.Fatalf("GenerateProbe: %v", err)
	}
	var sets []gcode.Command
	for _, c := range cmds {
		if c.Name == "G10" {
			sets = append(sets, c)
		}
	}
	if len(sets) != 2 {
		t.Fatalf("got %d G10 sets, want 2", len(sets))
	}
	if sets[0].Params["L"] != 20 || sets[0].Params["P"] != 2 {
		t.Errorf("G10 should be L20 P2, got %+v", sets[0].Params)
	}
	if _, ok := sets[0].Params["Z"]; !ok {
		t.Errorf("first set should zero Z, got %+v", sets[0].Params)
	}
	if _, ok := sets[1].Params["X"]; !ok {
		t.Errorf("second set should zero X, got %+v", sets[1].Params)
	}
}

// TestProbeNoOffsetWhenDisabled checks WorkOffset=0 (or an unset axis) emits no G10.
func TestProbeNoOffsetWhenDisabled(t *testing.T) {
	pts := []ProbePoint{{Approach: gcode.Vector3{X: 0, Y: 0, Z: 2}, Target: gcode.Vector3{X: 0, Y: 0, Z: -5}, SetAxis: "Z"}}
	cmds, _ := GenerateProbe(pts, ProbeParams{ProbeFeed: 50, WorkOffset: 0})
	for _, c := range cmds {
		if c.Name == "G10" {
			t.Errorf("WorkOffset 0 should emit no G10, found %+v", c.Params)
		}
	}
	// a point with no SetAxis also emits no G10 even when the offset is enabled.
	noAxis := []ProbePoint{{Approach: gcode.Vector3{X: 0, Y: 0, Z: 2}, Target: gcode.Vector3{X: 0, Y: 0, Z: -5}}}
	cmds2, _ := GenerateProbe(noAxis, ProbeParams{ProbeFeed: 50, WorkOffset: 1})
	for _, c := range cmds2 {
		if c.Name == "G10" {
			t.Errorf("a touch with no SetAxis should emit no G10, found %+v", c.Params)
		}
	}
}

// TestProbeErrors covers the degenerate feed / empty / no-direction cases.
func TestProbeErrors(t *testing.T) {
	if _, err := GenerateProbe(cornerProbe(), ProbeParams{ProbeFeed: 0}); err == nil {
		t.Error("a non-positive probe feed must error")
	}
	if _, err := GenerateProbe(nil, ProbeParams{ProbeFeed: 50}); err == nil {
		t.Error("no probe points must error")
	}
	stationary := []ProbePoint{{Approach: gcode.Vector3{X: 1, Y: 1, Z: 1}, Target: gcode.Vector3{X: 1, Y: 1, Z: 1}}}
	if _, err := GenerateProbe(stationary, ProbeParams{ProbeFeed: 50}); err == nil {
		t.Error("a probe with no direction must error")
	}
}
