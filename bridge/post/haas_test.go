// SPDX-License-Identifier: GPL-2.0-only

package post

import (
	"strings"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// haasObject wraps parsed lines as a single labelled object.
func haasObject(label string, lines ...string) []Object {
	cmds := make([]gcode.Command, 0, len(lines))
	for _, l := range lines {
		cmds = append(cmds, gcode.ParseCommand(l))
	}
	return []Object{{Label: label, Path: gcode.NewPath(cmds)}}
}

// TestHaasEmptyProgram checks the tape wrapper, five-digit O-number, safe-start block, and the
// G28 home / M30 footer.
func TestHaasEmptyProgram(t *testing.T) {
	empty := []Object{{Label: "op", Path: gcode.NewPath(nil)}}
	want := "%\nN10 O00001 (Exported by Oblikovati)\nN20 G00 G17 G21 G40 G49 G80 G90 G54\n" +
		"N30 (op)\nN40 M05\nN50 G28 G91 Z0.\nN60 G90\nN70 M30\n%\n"
	if got := ExportHaas(empty, ""); got != want {
		t.Errorf("default:\n got %q\nwant %q", got, want)
	}
	wantBare := "%\nO00001\nG00 G17 G21 G40 G49 G80 G90 G54\nM05\nG28 G91 Z0.\nG90\nM30\n%\n"
	if got := ExportHaas(empty, "--no-comments --no-sequence-numbers"); got != wantBare {
		t.Errorf("bare:\n got %q\nwant %q", got, wantBare)
	}
}

// TestHaasMotionAndDrill checks motion formatting (4-decimal, feeds suppressed on rapids) and
// canned drill-cycle pass-through.
func TestHaasMotionAndDrill(t *testing.T) {
	out := ExportHaas(haasObject("op", "G0 X0 Y0", "G81 X10 Y10 Z-3 R2 F50"), "--no-sequence-numbers --no-comments")
	if !strings.Contains(out, "G0 X0.0000 Y0.0000\n") {
		t.Errorf("missing 4-decimal rapid (no feed):\n%s", out)
	}
	if !strings.Contains(out, "G81 X10.0000 Y10.0000 Z-3.0000 R2.0000 F50.0000\n") {
		t.Errorf("canned drill cycle not preserved:\n%s", out)
	}
}

// TestHaasProgramNumberAndInches checks the five-digit O-number and inch unit.
func TestHaasProgramNumberAndInches(t *testing.T) {
	out := ExportHaas(haasObject("op", "G1 X25.4 F100"), "--program-number=42 --inches --no-sequence-numbers --no-comments")
	if !strings.HasPrefix(out, "%\nO00042\nG00 G17 G20 G40 G49 G80 G90 G54\n") {
		t.Errorf("O-number / inch safe-start wrong:\n%s", out)
	}
	if !strings.Contains(out, "G1 X1.0000 F100.0000\n") {
		t.Errorf("inch scaling wrong:\n%s", out)
	}
}

// TestHaasDispatch checks the post is reachable by name.
func TestHaasDispatch(t *testing.T) {
	out, err := Export("haas", haasObject("op", "G1 X1 Y2 F100"), "--no-sequence-numbers --no-comments")
	if err != nil {
		t.Fatalf("Export haas: %v", err)
	}
	if !strings.HasPrefix(out, "%\nO00001\n") {
		t.Errorf("dispatch did not route to the Haas post:\n%s", out)
	}
}
