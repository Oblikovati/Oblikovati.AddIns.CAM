// SPDX-License-Identifier: GPL-2.0-only

package post

import (
	"strings"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// klartextObject wraps parsed lines as a single labelled object.
func klartextObject(label string, lines ...string) []Object {
	cmds := make([]gcode.Command, 0, len(lines))
	for _, l := range lines {
		cmds = append(cmds, gcode.ParseCommand(l))
	}
	return []Object{{Label: label, Path: gcode.NewPath(cmds)}}
}

// klartextLines splits the Klartext output into trimmed, non-empty block lines.
func klartextLines(out string) []string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

// TestHeidenhainProgramFrame checks the numbered BEGIN/END PGM frame and the program name + unit.
func TestHeidenhainProgramFrame(t *testing.T) {
	out := ExportHeidenhain(klartextObject("op"), "--program=PART1")
	lines := klartextLines(out)
	if lines[0] != "0 BEGIN PGM PART1 MM" {
		t.Errorf("first block = %q, want the numbered BEGIN PGM", lines[0])
	}
	last := lines[len(lines)-1]
	if !strings.HasSuffix(last, "END PGM PART1 MM") {
		t.Errorf("last block = %q, want END PGM PART1 MM", last)
	}
	// Block numbers increase from 0.
	if !strings.HasPrefix(lines[0], "0 ") {
		t.Errorf("blocks must be numbered from 0, got %q", lines[0])
	}
}

// TestHeidenhainMotionAndToolCall checks tool calls, signed-coordinate L moves with FMAX rapids,
// and the cutting feed format.
func TestHeidenhainMotionAndToolCall(t *testing.T) {
	out := ExportHeidenhain(klartextObject("op",
		"M6 T3", "M3 S2400", "G0 Z15", "G1 Z-5 F100", "G1 X10 Y-4 F300", "M5"), "--no-comments")
	for _, want := range []string{
		"TOOL CALL 3 Z S2400",
		"M3",
		"L Z+15.000 R0 FMAX",         // rapid
		"L Z-5.000 R0 F100",          // plunge feed
		"L X+10.000 Y-4.000 R0 F300", // signed coordinates
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Klartext should contain %q\nfull:\n%s", want, out)
		}
	}
}

// TestHeidenhainArc checks a G2 becomes a CC centre block plus a clockwise C block.
func TestHeidenhainArc(t *testing.T) {
	out := ExportHeidenhain(klartextObject("op", "G1 X10 Y0 F200", "G2 X20 Y10 I0 J10 F200"), "--no-comments")
	if !strings.Contains(out, "CC X+10.000 Y+10.000") {
		t.Errorf("arc should set the circle centre from I/J, got:\n%s", out)
	}
	if !strings.Contains(out, "C X+20.000 Y+10.000") || !strings.Contains(out, "DR-") {
		t.Errorf("clockwise arc should be a C … DR- block, got:\n%s", out)
	}
}

// TestHeidenhainDrillExpansion checks a canned drill cycle is expanded to explicit L moves (no
// G81 survives), since this post has no fixed cycle.
func TestHeidenhainDrillExpansion(t *testing.T) {
	out := ExportHeidenhain(klartextObject("op", "G81 X5 Y5 Z-8 R2 F80"), "--no-comments")
	if strings.Contains(out, "G81") {
		t.Error("the drill cycle must be expanded, not emitted as G81")
	}
	for _, want := range []string{
		"L X+5.000 Y+5.000 R0 FMAX", // rapid over the hole
		"L Z+2.000 R0 FMAX",         // to the R plane
		"L Z-8.000 R0 F80",          // feed to depth
	} {
		if !strings.Contains(out, want) {
			t.Errorf("drill expansion should contain %q\nfull:\n%s", want, out)
		}
	}
}

// TestHeidenhainInches switches the program unit and converts coordinates.
func TestHeidenhainInches(t *testing.T) {
	out := ExportHeidenhain(klartextObject("op", "G1 X25.4 F100"), "--inches --no-comments")
	if !strings.Contains(out, "BEGIN PGM OBLIKOVATI INCH") {
		t.Errorf("inches mode should declare INCH units, got:\n%s", out)
	}
	if !strings.Contains(out, "X+1.0000") { // 25.4 mm = 1 inch
		t.Errorf("25.4 mm should convert to +1.0000 in, got:\n%s", out)
	}
}

// TestHeidenhainViaDispatch routes through the Export dispatcher.
func TestHeidenhainViaDispatch(t *testing.T) {
	out, err := Export("heidenhain", klartextObject("op", "G1 X1 Y2 F100"), "--no-comments")
	if err != nil {
		t.Fatalf("Export heidenhain: %v", err)
	}
	if !strings.Contains(out, "BEGIN PGM") {
		t.Errorf("dispatch should produce a Klartext program, got:\n%s", out)
	}
}
