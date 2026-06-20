// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// TestToolLengthProbe checks the cycle probes down at the setter and sets the tool-length offset
// (G10 L1) for the given tool at the setter top.
func TestToolLengthProbe(t *testing.T) {
	cmds, err := GenerateToolLengthProbe(-50, -50, 25, 3, ProbeParams{ClearanceZ: 60, ProbeFeed: 40})
	if err != nil {
		t.Fatalf("GenerateToolLengthProbe: %v", err)
	}
	var probe, set gcode.Command
	for _, c := range cmds {
		switch c.Name {
		case "G38.2":
			probe = c
		case "G10":
			set = c
		}
	}
	if probe.Name == "" || probe.Params["F"] != 40 {
		t.Errorf("expected a G38.2 probe at the probe feed, got %+v", probe)
	}
	// the probe descends below the setter top (25) to ensure contact.
	if probe.Params["Z"] >= 25 {
		t.Errorf("probe should descend below the setter top 25, got Z%g", probe.Params["Z"])
	}
	// the offset set is G10 L1 (tool length) for tool 3 at the setter top.
	if set.Params["L"] != 1 || set.Params["P"] != 3 || set.Params["Z"] != 25 {
		t.Errorf("expected G10 L1 P3 Z25, got %+v", set.Params)
	}
}

// TestToolLengthProbeErrors covers the degenerate feed / tool-number cases.
func TestToolLengthProbeErrors(t *testing.T) {
	if _, err := GenerateToolLengthProbe(0, 0, 25, 1, ProbeParams{ProbeFeed: 0}); err == nil {
		t.Error("a non-positive probe feed must error")
	}
	if _, err := GenerateToolLengthProbe(0, 0, 25, 0, ProbeParams{ProbeFeed: 40}); err == nil {
		t.Error("a non-positive tool number must error")
	}
}
