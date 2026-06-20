// SPDX-License-Identifier: GPL-2.0-only

package post

import (
	"strings"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// grblObject wraps parsed lines as the "testpath" object the GRBL oracle uses.
func grblObject(lines ...string) []Object {
	cmds := make([]gcode.Command, 0, len(lines))
	for _, l := range lines {
		cmds = append(cmds, gcode.ParseCommand(l))
	}
	return []Object{{Label: "testpath", Path: gcode.NewPath(cmds)}}
}

// grblLine renders and returns the given line index.
func grblLine(t *testing.T, idx int, args string, lines ...string) string {
	t.Helper()
	out := ExportGRBL(grblObject(lines...), args)
	split := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if idx >= len(split) {
		t.Fatalf("output has %d lines, want index %d:\n%s", len(split), idx, out)
	}
	return split[idx]
}

// TestGRBLEmptyPath ports TestGrblLegacyPost.test000.
func TestGRBLEmptyPath(t *testing.T) {
	empty := []Object{{Label: "testpath", Path: gcode.NewPath(nil)}}

	withHeader := ExportGRBL(empty, "--no-show-editor")
	if got := len(strings.Split(strings.TrimRight(withHeader, "\n"), "\n")); got != 13 {
		t.Errorf("with header: %d lines, want 13:\n%s", got, withHeader)
	}

	wantComments := "(Begin preamble)\nG17 G90\nG21\n(Begin operation: testpath)\n(Path: testpath)\n" +
		"(Finish operation: testpath)\n(Begin postamble)\nM5\nG17 G90\nM2\n"
	if got := ExportGRBL(empty, "--no-header --no-show-editor"); got != wantComments {
		t.Errorf("no-header:\n got %q\nwant %q", got, wantComments)
	}

	wantNoComments := "G17 G90\nG21\nM5\nG17 G90\nM2\n"
	if got := ExportGRBL(empty, "--no-header --no-comments --no-show-editor"); got != wantNoComments {
		t.Errorf("no-comments:\n got %q\nwant %q", got, wantNoComments)
	}
}

// TestGRBLPrecisionAndInches ports test010/test050 (GRBL lines carry no trailing space).
func TestGRBLPrecisionAndInches(t *testing.T) {
	cases := []struct{ args, want string }{
		{"--no-header --no-show-editor", "G0 X10.000 Y20.000 Z30.000"},
		{"--no-header --precision=2 --no-show-editor", "G0 X10.00 Y20.00 Z30.00"},
		{"--no-header --inches --no-show-editor", "G0 X0.3937 Y0.7874 Z1.1811"},
	}
	for _, c := range cases {
		if got := grblLine(t, 5, c.args, "G0 X10 Y20 Z30"); got != c.want {
			t.Errorf("args %q: got %q, want %q", c.args, got, c.want)
		}
	}
	if got := grblLine(t, 2, "--no-header --inches --no-show-editor", "G0 X10 Y20 Z30"); got != "G20" {
		t.Errorf("inches units line = %q, want G20", got)
	}
}

// TestGRBLLineNumbers ports test020 (single space after N, counter returned before increment).
func TestGRBLLineNumbers(t *testing.T) {
	if got := grblLine(t, 5, "--no-header --line-numbers --no-show-editor", "G0 X10 Y20 Z30"); got != "N150 G0 X10.000 Y20.000 Z30.000" {
		t.Errorf("got %q, want %q", got, "N150 G0 X10.000 Y20.000 Z30.000")
	}
}

// TestGRBLPreamblePostamble ports test030/test040.
func TestGRBLPreamblePostamble(t *testing.T) {
	empty := []Object{{Label: "testpath", Path: gcode.NewPath(nil)}}
	pre := ExportGRBL(empty, "--no-header --no-comments --preamble='G18 G55\\n' --no-show-editor")
	if first := strings.Split(pre, "\n")[0]; first != "G18 G55" {
		t.Errorf("preamble first line = %q, want G18 G55", first)
	}
	post := ExportGRBL(empty, "--no-header --no-comments --postamble='G0 Z50\\nM2' --no-show-editor")
	lines := strings.Split(strings.TrimRight(post, "\n"), "\n")
	if lines[len(lines)-2] != "G0 Z50" || lines[len(lines)-1] != "M2" {
		t.Errorf("postamble tail = %q / %q, want G0 Z50 / M2", lines[len(lines)-2], lines[len(lines)-1])
	}
}

// TestGRBLToolChange ports test080: M6 is commented out (GRBL ignores it).
func TestGRBLToolChange(t *testing.T) {
	if got := grblLine(t, 6, "--no-header --no-show-editor", "M6 T2", "M3 S3000"); got != "( M6 T2 )" {
		t.Errorf("toolchange = %q, want %q", got, "( M6 T2 )")
	}
	if got := grblLine(t, 7, "--no-header --no-show-editor", "M6 T2", "M3 S3000"); got != "M3 S3000" {
		t.Errorf("spindle = %q, want M3 S3000", got)
	}
}

// TestGRBLComment ports test090.
func TestGRBLComment(t *testing.T) {
	if got := grblLine(t, 5, "--no-header --no-show-editor", "(comment)"); got != "(comment)" {
		t.Errorf("got %q, want (comment)", got)
	}
}

// TestGRBLDrillTranslateG83 verifies a peck cycle expands into stepped plunges with
// retracts between pecks (Q=3 over a 0..-9 hole gives three pecks).
func TestGRBLDrillTranslateG83(t *testing.T) {
	obj := []Object{{Label: "p", Path: gcode.NewPath([]gcode.Command{
		gcode.ParseCommand("G0 Z15"),
		gcode.NewCommand("G83", map[string]float64{"X": 0, "Y": 0, "Z": -9, "R": 12, "Q": 3, "F": 100}),
	})}}
	out := ExportGRBL(obj, "--no-header --no-comments --no-show-editor")
	plunges := strings.Count(out, "G1 Z")
	if plunges < 3 {
		t.Errorf("G83 peck should produce >=3 plunge moves, got %d:\n%s", plunges, out)
	}
	if !strings.Contains(out, "G1 Z-9.000 F100.00") {
		t.Errorf("final peck to full depth missing:\n%s", out)
	}
}

// TestExportDispatch covers the post-name dispatcher and its unknown-name error.
func TestExportDispatch(t *testing.T) {
	if _, err := Export("linuxcnc", grblObject("G0 X1"), "--no-header"); err != nil {
		t.Errorf("linuxcnc dispatch: %v", err)
	}
	if _, err := Export("grbl", grblObject("G0 X1"), "--no-header"); err != nil {
		t.Errorf("grbl dispatch: %v", err)
	}
	if _, err := Export("nonesuch", nil, ""); err == nil {
		t.Error("unknown post must error")
	}
}

// TestGRBLDrillTranslateG81 verifies a G81 canned cycle is expanded into the explicit
// position/plunge/retract moves GRBL requires (GRBL has no canned cycles). With the tool
// already at the clearance plane (Z15) above R12, drilling to Z0: rapid over XY, plunge to
// Z0 at feed, retract to the G98 return level (the prior Z15).
func TestGRBLDrillTranslateG81(t *testing.T) {
	obj := []Object{{Label: "p", Path: gcode.NewPath([]gcode.Command{
		gcode.ParseCommand("G0 Z15"),
		gcode.ParseCommand("G0 X5 Y6"),
		gcode.NewCommand("G81", map[string]float64{"X": 5, "Y": 6, "Z": 0, "R": 12, "F": 100}),
	})}}
	out := ExportGRBL(obj, "--no-header --no-comments --no-show-editor")
	// Drilling moves must contain the plunge to Z0 at the feed and a retract above R.
	for _, want := range []string{"G1 Z0.000 F100.00", "G0 X5.000 Y6.000", "G0 Z15.000"} {
		if !strings.Contains(out, want) {
			t.Errorf("drill translation missing %q in:\n%s", want, out)
		}
	}
	// The canned-cycle code itself must NOT appear (it was translated away).
	if strings.Contains(out, "G81") {
		t.Errorf("G81 should have been translated out:\n%s", out)
	}
}
