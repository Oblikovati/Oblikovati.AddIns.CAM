// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// TestGenerateTapRightHand checks a plain tap emits a single G84 cycle carrying the hole XY, the
// bottom Z, the top as the R plane, and the synchronised feed.
func TestGenerateTapRightHand(t *testing.T) {
	cmds, err := GenerateTap(
		gcode.Vector3{X: 3, Y: 4, Z: 10}, gcode.Vector3{X: 3, Y: 4, Z: 0},
		750, TapParams{Pitch: 1.5},
	)
	if err != nil {
		t.Fatalf("GenerateTap: %v", err)
	}
	if len(cmds) != 1 || cmds[0].Name != "G84" {
		t.Fatalf("want one G84 command, got %v", cmds)
	}
	for addr, want := range map[string]float64{"X": 3, "Y": 4, "Z": 0, "R": 10, "F": 750} {
		if got := cmds[0].Params[addr]; got != want {
			t.Errorf("G84 %s = %g, want %g", addr, got, want)
		}
	}
	if _, ok := cmds[0].Params["P"]; ok {
		t.Error("a tap without dwell must not carry a P address")
	}
}

// TestGenerateTapLeftHand checks a left-hand tap uses the reverse cycle G74.
func TestGenerateTapLeftHand(t *testing.T) {
	cmds, err := GenerateTap(
		gcode.Vector3{X: 0, Y: 0, Z: 5}, gcode.Vector3{X: 0, Y: 0, Z: -5},
		500, TapParams{Pitch: 1.0, LeftHand: true},
	)
	if err != nil {
		t.Fatalf("GenerateTap: %v", err)
	}
	if cmds[0].Name != "G74" {
		t.Errorf("a left-hand tap must emit G74, got %s", cmds[0].Name)
	}
}

// TestGenerateTapDwell checks a requested bottom dwell appears as the P address.
func TestGenerateTapDwell(t *testing.T) {
	cmds, err := GenerateTap(
		gcode.Vector3{X: 0, Y: 0, Z: 10}, gcode.Vector3{X: 0, Y: 0, Z: 0},
		600, TapParams{Pitch: 1.25, DwellTime: 0.5},
	)
	if err != nil {
		t.Fatalf("GenerateTap: %v", err)
	}
	if cmds[0].Params["P"] != 0.5 {
		t.Errorf("dwell P = %g, want 0.5", cmds[0].Params["P"])
	}
}

// TestGenerateTapErrors covers the degenerate parameter and geometry cases.
func TestGenerateTapErrors(t *testing.T) {
	good := gcode.Vector3{X: 0, Y: 0, Z: 10}
	bottom := gcode.Vector3{X: 0, Y: 0, Z: 0}
	cases := []struct {
		name        string
		start, end  gcode.Vector3
		feed, pitch float64
	}{
		{"zero pitch", good, bottom, 750, 0},
		{"zero feed", good, bottom, 0, 1.5},
		{"not Z-aligned", good, gcode.Vector3{X: 2, Y: 0, Z: 0}, 750, 1.5},
		{"start below end", bottom, good, 750, 1.5},
	}
	for _, c := range cases {
		if _, err := GenerateTap(c.start, c.end, c.feed, TapParams{Pitch: c.pitch}); err == nil {
			t.Errorf("%s: expected an error", c.name)
		}
	}
}
