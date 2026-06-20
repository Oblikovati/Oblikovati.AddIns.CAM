// SPDX-License-Identifier: GPL-2.0-only

package post

import (
	"strings"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// marlinObject wraps parsed lines as a single labelled object.
func marlinObject(label string, lines ...string) []Object {
	cmds := make([]gcode.Command, 0, len(lines))
	for _, l := range lines {
		cmds = append(cmds, gcode.ParseCommand(l))
	}
	return []Object{{Label: label, Path: gcode.NewPath(cmds)}}
}

// TestMarlinEmptyProgram checks the semicolon-commented metric/absolute skeleton.
func TestMarlinEmptyProgram(t *testing.T) {
	empty := []Object{{Label: "op", Path: gcode.NewPath(nil)}}
	want := "; Exported by Oblikovati\n; Post Processor: marlin\nG21\nG90\n; op\nM5\n; End\n"
	if got := ExportMarlin(empty, ""); got != want {
		t.Errorf("default:\n got %q\nwant %q", got, want)
	}
	if got := ExportMarlin(empty, "--no-comments"); got != "G21\nG90\nM5\n" {
		t.Errorf("no-comments: got %q", got)
	}
}

// TestMarlinMotionAndComments checks rapid feeds are suppressed and path comments use ";".
func TestMarlinMotionAndComments(t *testing.T) {
	out := ExportMarlin(marlinObject("op", "G0 X0 Y0", "G1 Z-5 F100", "(hello)"), "")
	if !strings.Contains(out, "G0 X0.000 Y0.000\n") {
		t.Errorf("missing rapid (and it must carry no feed):\n%s", out)
	}
	if strings.Contains(out, "G0 X0.000 Y0.000 F") {
		t.Errorf("rapid should not carry a feed:\n%s", out)
	}
	if !strings.Contains(out, "G1 Z-5.000 F100.000\n") {
		t.Errorf("missing formatted feed move:\n%s", out)
	}
	if !strings.Contains(out, "; hello\n") {
		t.Errorf("path comment should become a ';' comment:\n%s", out)
	}
}

// TestMarlinTranslatesDrill checks a canned drill cycle expands to explicit moves (Marlin has no
// canned cycles).
func TestMarlinTranslatesDrill(t *testing.T) {
	out := ExportMarlin(marlinObject("op", "G81 X10 Y10 Z-3 R2 F50"), "--no-comments")
	want := "G0 X10.000 Y10.000\nG0 Z2.000\nG1 Z-3.000 F50.000\nG0 Z2.000\n"
	if !strings.Contains(out, want) {
		t.Errorf("drill cycle not translated:\n got %q\nwant it to contain %q", out, want)
	}
	if strings.Contains(out, "G81") {
		t.Errorf("a raw G81 survived (Marlin has no canned cycles):\n%s", out)
	}
}

// TestMarlinToolChangePauses checks an M6 becomes an M0 pause.
func TestMarlinToolChangePauses(t *testing.T) {
	out := ExportMarlin(marlinObject("op", "M6 T2"), "")
	if !strings.Contains(out, "M0\n") {
		t.Errorf("tool change should pause with M0:\n%s", out)
	}
}

// TestMarlinDispatch checks the post is reachable by name.
func TestMarlinDispatch(t *testing.T) {
	out, err := Export("marlin", marlinObject("op", "G1 X1 Y2 F100"), "--no-comments")
	if err != nil {
		t.Fatalf("Export marlin: %v", err)
	}
	if !strings.HasPrefix(out, "G21\nG90\n") {
		t.Errorf("dispatch did not route to the Marlin post:\n%s", out)
	}
}
