// SPDX-License-Identifier: GPL-2.0-only

package post

import (
	"strings"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// fanucObject wraps parsed lines as a single labelled object.
func fanucObject(label string, lines ...string) []Object {
	cmds := make([]gcode.Command, 0, len(lines))
	for _, l := range lines {
		cmds = append(cmds, gcode.ParseCommand(l))
	}
	return []Object{{Label: label, Path: gcode.NewPath(cmds)}}
}

// TestFanucEmptyProgram checks the tape-wrapped, O-numbered, sequence-numbered skeleton.
func TestFanucEmptyProgram(t *testing.T) {
	empty := []Object{{Label: "testpath", Path: gcode.NewPath(nil)}}
	want := "%\nN10 O0001 (Exported by Oblikovati)\nN20 G17 G21 G90 G94 G54\nN30 (testpath)\nN40 M05\nN50 M30\n%\n"
	if got := ExportFanuc(empty, ""); got != want {
		t.Errorf("default:\n got %q\nwant %q", got, want)
	}
	wantBare := "%\nO0001\nG17 G21 G90 G94 G54\nM05\nM30\n%\n"
	if got := ExportFanuc(empty, "--no-comments --no-sequence-numbers"); got != wantBare {
		t.Errorf("bare:\n got %q\nwant %q", got, wantBare)
	}
}

// TestFanucMotionAndDrill checks motion formatting (feeds suppressed on rapids) and that canned
// drill cycles pass through unchanged (Fanuc keeps them, unlike GRBL).
func TestFanucMotionAndDrill(t *testing.T) {
	out := ExportFanuc(fanucObject("op",
		"G0 X0 Y0",
		"G1 Z-5 F100",
		"G81 X10 Y10 Z-3 R2 F50",
	), "--no-sequence-numbers --no-comments")
	if strings.Contains(out, "F") && strings.Contains(out, "G0 X0.000 Y0.000 F") {
		t.Errorf("rapid should carry no feed:\n%s", out)
	}
	if !strings.Contains(out, "G0 X0.000 Y0.000\n") {
		t.Errorf("missing formatted rapid:\n%s", out)
	}
	if !strings.Contains(out, "G81 X10.000 Y10.000 Z-3.000 R2.000 F50.000\n") {
		t.Errorf("canned drill cycle not preserved:\n%s", out)
	}
}

// TestFanucProgramNumberAndInches checks the O-number and inch scaling.
func TestFanucProgramNumberAndInches(t *testing.T) {
	out := ExportFanuc(fanucObject("op", "G1 X25.4 Y50.8 F100"), "--program-number=42 --inches --no-sequence-numbers --no-comments")
	if !strings.HasPrefix(out, "%\nO0042\nG17 G20 G90 G94 G54\n") {
		t.Errorf("O-number / inch unit header wrong:\n%s", out)
	}
	// 25.4 mm = 1.0000 in at precision 4.
	if !strings.Contains(out, "G1 X1.0000 Y2.0000 F100.0000\n") {
		t.Errorf("inch scaling wrong:\n%s", out)
	}
}

// TestFanucDispatch checks the post is reachable by name.
func TestFanucDispatch(t *testing.T) {
	out, err := Export("fanuc", fanucObject("op", "G1 X1 Y2 F100"), "--no-sequence-numbers --no-comments")
	if err != nil {
		t.Fatalf("Export fanuc: %v", err)
	}
	if !strings.HasPrefix(out, "%\nO0001\n") {
		t.Errorf("dispatch did not route to the Fanuc post:\n%s", out)
	}
}
