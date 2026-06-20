// SPDX-License-Identifier: GPL-2.0-only

package post

import (
	"strings"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// pathObject wraps one or more G-code lines (parsed) as a single "testpath" object, the
// fixture shape FreeCAD's TestLinuxCNCLegacyPost uses.
func pathObject(lines ...string) []Object {
	cmds := make([]gcode.Command, 0, len(lines))
	for _, l := range lines {
		cmds = append(cmds, gcode.ParseCommand(l))
	}
	return []Object{{Label: "testpath", Path: gcode.NewPath(cmds)}}
}

// sixthLine renders and returns line index 5, the line the upstream compare_sixth_line
// helper checks (the first real command after preamble + operation comments).
func sixthLine(t *testing.T, line, args string) string {
	t.Helper()
	out := ExportLinuxCNC(pathObject(line), args)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) <= 5 {
		t.Fatalf("output has only %d lines:\n%s", len(lines), out)
	}
	return lines[5]
}

// TestLinuxCNCEmptyPath ports test000: header line count and the two header-less renders.
func TestLinuxCNCEmptyPath(t *testing.T) {
	empty := []Object{{Label: "testpath", Path: gcode.NewPath(nil)}}

	withHeader := ExportLinuxCNC(empty, "--no-show-editor")
	if got := len(strings.Split(strings.TrimRight(withHeader, "\n"), "\n")); got != 13 {
		t.Errorf("with header: %d lines, want 13:\n%s", got, withHeader)
	}

	wantComments := "(begin preamble)\nG17 G54 G40 G49 G80 G90\nG21\n(begin operation: testpath)\n" +
		"(machine units: mm/min)\n(finish operation: testpath)\n(begin postamble)\nM05\nG17 G54 G90 G80 G40\nM2\n"
	if got := ExportLinuxCNC(empty, "--no-header --no-show-editor"); got != wantComments {
		t.Errorf("no-header:\n got %q\nwant %q", got, wantComments)
	}

	wantNoComments := "G17 G54 G40 G49 G80 G90\nG21\nM05\nG17 G54 G90 G80 G40\nM2\n"
	if got := ExportLinuxCNC(empty, "--no-header --no-comments --no-show-editor"); got != wantNoComments {
		t.Errorf("no-comments:\n got %q\nwant %q", got, wantNoComments)
	}
}

// TestLinuxCNCPrecisionAndInches ports test010/test050.
func TestLinuxCNCPrecisionAndInches(t *testing.T) {
	cases := []struct{ line, args, want string }{
		{"G0 X10 Y20 Z30", "--no-header --no-show-editor", "G0 X10.000 Y20.000 Z30.000 "},
		{"G0 X10 Y20 Z30", "--no-header --precision=2 --no-show-editor", "G0 X10.00 Y20.00 Z30.00 "},
		{"G0 X10 Y20 Z30", "--no-header --inches --no-show-editor", "G0 X0.3937 Y0.7874 Z1.1811 "},
	}
	for _, c := range cases {
		if got := sixthLine(t, c.line, c.args); got != c.want {
			t.Errorf("args %q: got %q, want %q", c.args, got, c.want)
		}
	}
	// --inches also emits the G20 units line at index 2.
	out := ExportLinuxCNC(pathObject("G0 X10 Y20 Z30"), "--no-header --inches --no-show-editor")
	if line2 := strings.Split(out, "\n")[2]; line2 != "G20" {
		t.Errorf("inches units line = %q, want G20", line2)
	}
}

// TestLinuxCNCLineNumbers ports test020.
func TestLinuxCNCLineNumbers(t *testing.T) {
	if got := sixthLine(t, "G0 X10 Y20 Z30", "--no-header --line-numbers --no-show-editor"); got != "N160  G0 X10.000 Y20.000 Z30.000 " {
		t.Errorf("got %q, want %q", got, "N160  G0 X10.000 Y20.000 Z30.000 ")
	}
}

// TestLinuxCNCPreamblePostamble ports test030/test040.
func TestLinuxCNCPreamblePostamble(t *testing.T) {
	empty := []Object{{Label: "testpath", Path: gcode.NewPath(nil)}}

	pre := ExportLinuxCNC(empty, "--no-header --no-comments --preamble='G18 G55' --no-show-editor")
	if first := strings.Split(pre, "\n")[0]; first != "G18 G55" {
		t.Errorf("preamble first line = %q, want G18 G55", first)
	}

	post := ExportLinuxCNC(empty, "--no-header --no-comments --postamble='G0 Z50\\nM2' --no-show-editor")
	lines := strings.Split(strings.TrimRight(post, "\n"), "\n")
	if lines[len(lines)-2] != "G0 Z50" || lines[len(lines)-1] != "M2" {
		t.Errorf("postamble tail = %q / %q, want G0 Z50 / M2", lines[len(lines)-2], lines[len(lines)-1])
	}
}

// TestLinuxCNCModal ports test060: a repeated command name is suppressed.
func TestLinuxCNCModal(t *testing.T) {
	out := ExportLinuxCNC([]Object{{Label: "testpath", Path: gcode.NewPath([]gcode.Command{
		gcode.ParseCommand("G0 X10 Y20 Z30"), gcode.ParseCommand("G0 X10 Y30 Z30"),
	})}}, "--no-header --modal --no-show-editor")
	if got := strings.Split(out, "\n")[6]; got != "X10.000 Y30.000 Z30.000 " {
		t.Errorf("modal line = %q, want %q", got, "X10.000 Y30.000 Z30.000 ")
	}
}

// TestLinuxCNCAxisModal ports test070: an axis equal to the previous is suppressed.
func TestLinuxCNCAxisModal(t *testing.T) {
	out := ExportLinuxCNC([]Object{{Label: "testpath", Path: gcode.NewPath([]gcode.Command{
		gcode.ParseCommand("G0 X10 Y20 Z30"), gcode.ParseCommand("G0 X10 Y30 Z30"),
	})}}, "--no-header --axis-modal --no-show-editor")
	if got := strings.Split(out, "\n")[6]; got != "G0 Y30.000 " {
		t.Errorf("axis-modal line = %q, want %q", got, "G0 Y30.000 ")
	}
}

// TestLinuxCNCToolChange ports test080: M6 emits M5, the tool-change line, and (with TLO)
// a G43 Hn line; --no-tlo suppresses the G43.
func TestLinuxCNCToolChange(t *testing.T) {
	objs := []Object{{Label: "testpath", Path: gcode.NewPath([]gcode.Command{
		gcode.ParseCommand("M6 T2"), gcode.ParseCommand("M3 S3000"),
	})}}
	lines := strings.Split(ExportLinuxCNC(objs, "--no-header --no-show-editor"), "\n")
	for i, want := range map[int]string{5: "M5", 6: "M6 T2 ", 7: "G43 H2 ", 8: "M3 S3000 "} {
		if lines[i] != want {
			t.Errorf("line[%d] = %q, want %q", i, lines[i], want)
		}
	}
	noTLO := strings.Split(ExportLinuxCNC(objs, "--no-header --no-tlo --no-show-editor"), "\n")
	if noTLO[7] != "M3 S3000 " {
		t.Errorf("no-tlo line[7] = %q, want %q", noTLO[7], "M3 S3000 ")
	}
}

// TestLinuxCNCComment ports test090.
func TestLinuxCNCComment(t *testing.T) {
	if got := sixthLine(t, "(comment)", "--no-header --no-show-editor"); got != "(comment) " {
		t.Errorf("got %q, want %q", got, "(comment) ")
	}
}

// TestLinuxCNCABCAxes ports test100: rotary axes are emitted with the active precision and
// are not unit-converted (degrees).
func TestLinuxCNCABCAxes(t *testing.T) {
	if got := sixthLine(t, "G1 X10 Y20 Z30 A40 B50 C60", "--no-header --no-show-editor"); got != "G1 X10.000 Y20.000 Z30.000 A40.000 B50.000 C60.000 " {
		t.Errorf("metric ABC = %q", got)
	}
	if got := sixthLine(t, "G1 X10 Y20 Z30 A40 B50 C60", "--no-header --inches --no-show-editor"); got != "G1 X0.3937 Y0.7874 Z1.1811 A40.0000 B50.0000 C60.0000 " {
		t.Errorf("inch ABC = %q", got)
	}
}
