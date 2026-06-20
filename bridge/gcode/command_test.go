// SPDX-License-Identifier: GPL-2.0-only

package gcode

import "testing"

// TestParseCommand covers a move line, a comment line, an empty line, and malformed tokens.
func TestParseCommand(t *testing.T) {
	move := ParseCommand("G1 X10 Y-2.5 z3 F100 bad")
	if move.Name != "G1" {
		t.Errorf("name = %q, want G1", move.Name)
	}
	for addr, want := range map[string]float64{"X": 10, "Y": -2.5, "Z": 3, "F": 100} {
		if got := move.Params[addr]; got != want {
			t.Errorf("param %s = %g, want %g", addr, got, want)
		}
	}
	if _, ok := move.Params["BAD"]; ok {
		t.Error("the unparseable token 'bad' must be skipped")
	}

	if c := ParseCommand("(a comment)"); c.Name != "(a comment)" || len(c.Params) != 0 {
		t.Errorf("comment parse = %+v", c)
	}
	if c := ParseCommand("   "); c.Name != "" {
		t.Errorf("blank parse name = %q, want empty", c.Name)
	}
}

// TestToGCode covers the canonical rendering and the no-param shortcut.
func TestToGCode(t *testing.T) {
	c := NewCommand("G1", map[string]float64{"F": 100, "X": 10, "Y": 20, "Q": 1})
	// Canonical order puts X,Y before F, with Q after (extras sorted): "G1 X10 Y20 F100 Q1".
	if got := c.ToGCode(); got != "G1 X10 Y20 F100 Q1" {
		t.Errorf("ToGCode = %q", got)
	}
	if got := NewCommand("M2", nil).ToGCode(); got != "M2" {
		t.Errorf("no-param ToGCode = %q, want M2", got)
	}
}

// TestPathAdd covers the path constructor and append.
func TestPathAdd(t *testing.T) {
	p := NewPath(nil)
	p.Add(NewCommand("G0", nil))
	p.Add(NewCommand("G1", nil))
	if len(p.Commands) != 2 {
		t.Errorf("path length = %d, want 2", len(p.Commands))
	}
}
