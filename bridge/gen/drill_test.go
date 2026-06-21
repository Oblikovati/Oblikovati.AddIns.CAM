// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// topBottom is the canonical Z-aligned hole edge used across the drill generator tests:
// top at (0,0,10), bottom at (0,0,0).
func topBottom() (gcode.Vector3, gcode.Vector3) {
	return gcode.Vector3{X: 0, Y: 0, Z: 10}, gcode.Vector3{X: 0, Y: 0, Z: 0}
}

// onlyCommand runs the generator, asserts exactly one command came back, and returns it.
func onlyCommand(t *testing.T, p DrillParams) gcode.Command {
	t.Helper()
	start, end := topBottom()
	cmds, err := GenerateDrill(start, end, p)
	if err != nil {
		t.Fatalf("GenerateDrill: unexpected error: %v", err)
	}
	if len(cmds) != 1 {
		t.Fatalf("GenerateDrill: want 1 command, got %d", len(cmds))
	}
	return cmds[0]
}

// TestPlainDrillG81 checks a bare cycle is a G81 with the hole XY, bottom Z, and R defaulting
// to the start (top) Z.
func TestPlainDrillG81(t *testing.T) {
	c := onlyCommand(t, DrillParams{Repeat: 1})
	if c.Name != "G81" {
		t.Errorf("name = %q, want G81", c.Name)
	}
	for addr, want := range map[string]float64{"X": 0, "Y": 0, "Z": 0, "R": 10} {
		if got := c.Params[addr]; got != want {
			t.Errorf("param %s = %g, want %g", addr, got, want)
		}
	}
	if _, ok := c.Params["L"]; ok {
		t.Errorf("single pass must not emit L, got L=%g", c.Params["L"])
	}
}

// TestRepeatEmitsL: repeat>1 sets L; repeat<1 is an error (ports test00's repeat guard).
func TestRepeatEmitsL(t *testing.T) {
	c := onlyCommand(t, DrillParams{Repeat: 3})
	if got := c.Params["L"]; got != 3 {
		t.Errorf("L = %g, want 3", got)
	}
	start, end := topBottom()
	if _, err := GenerateDrill(start, end, DrillParams{Repeat: 0}); err == nil {
		t.Error("repeat 0 must error")
	}
}

// TestEdgeAlignmentGuard ports test10: a non-Z-aligned edge and a start-below-end edge
// both error.
func TestEdgeAlignmentGuard(t *testing.T) {
	// XY-skewed edge.
	if _, err := GenerateDrill(gcode.Vector3{X: 0, Y: 10, Z: 10}, gcode.Vector3{}, DrillParams{Repeat: 1}); err == nil {
		t.Error("non-Z-aligned edge must error")
	}
	// Start below end.
	if _, err := GenerateDrill(gcode.Vector3{X: 0, Y: 0, Z: 0}, gcode.Vector3{X: 0, Y: 0, Z: 10}, DrillParams{Repeat: 1}); err == nil {
		t.Error("start-below-end must error")
	}
}

// TestPeckG83 ports test20: a peck depth selects G83 carrying Q.
func TestPeckG83(t *testing.T) {
	c := onlyCommand(t, DrillParams{Repeat: 1, PeckDepth: 1.2})
	if c.Name != "G83" {
		t.Errorf("name = %q, want G83", c.Name)
	}
	if got := c.Params["Q"]; got != 1.2 {
		t.Errorf("Q = %g, want 1.2", got)
	}
}

// TestDwellG82 ports test30 + test71: a dwell selects G82 carrying P.
func TestDwellG82(t *testing.T) {
	c := onlyCommand(t, DrillParams{Repeat: 1, DwellTime: 0.5})
	if c.Name != "G82" {
		t.Errorf("name = %q, want G82", c.Name)
	}
	if got := c.Params["P"]; got != 0.5 {
		t.Errorf("P = %g, want 0.5", got)
	}
}

// TestRetractHeight ports test40/test41: an explicit retract height sets R; otherwise R
// defaults to the start (top) Z.
func TestRetractHeight(t *testing.T) {
	r := 20.0
	c := onlyCommand(t, DrillParams{Repeat: 1, RetractHeight: &r})
	if got := c.Params["R"]; got != 20.0 {
		t.Errorf("R = %g, want 20", got)
	}
	def := onlyCommand(t, DrillParams{Repeat: 1})
	if got := def.Params["R"]; got != 10.0 {
		t.Errorf("default R = %g, want 10 (start Z)", got)
	}
}

// TestDwellPeckExclusive ports test50: dwell + peck together is an error.
func TestDwellPeckExclusive(t *testing.T) {
	start, end := topBottom()
	if _, err := GenerateDrill(start, end, DrillParams{Repeat: 1, DwellTime: 1.0, PeckDepth: 1.0}); err == nil {
		t.Error("dwell + peck must error")
	}
}

// TestChipBreakG73 ports test60: peck + chipBreak selects G73.
func TestChipBreakG73(t *testing.T) {
	c := onlyCommand(t, DrillParams{Repeat: 1, PeckDepth: 1.0, ChipBreak: true})
	if c.Name != "G73" {
		t.Errorf("name = %q, want G73", c.Name)
	}
}

// TestFeedRetractG85 ports test70: feed-retract selects G85 and carries neither Q nor P.
func TestFeedRetractG85(t *testing.T) {
	c := onlyCommand(t, DrillParams{Repeat: 1, FeedRetract: true})
	if c.Name != "G85" {
		t.Errorf("name = %q, want G85", c.Name)
	}
	if _, ok := c.Params["Q"]; ok {
		t.Error("G85 must not carry Q")
	}
	if _, ok := c.Params["P"]; ok {
		t.Error("G85 must not carry P")
	}
}

// TestFeedRetractExclusions: feed-retract combined with dwell or peck is an error.
func TestFeedRetractExclusions(t *testing.T) {
	start, end := topBottom()
	if _, err := GenerateDrill(start, end, DrillParams{Repeat: 1, FeedRetract: true, DwellTime: 0.5}); err == nil {
		t.Error("feed-retract + dwell must error")
	}
	if _, err := GenerateDrill(start, end, DrillParams{Repeat: 1, FeedRetract: true, PeckDepth: 0.5}); err == nil {
		t.Error("feed-retract + peck must error")
	}
}
