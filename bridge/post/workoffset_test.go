// SPDX-License-Identifier: GPL-2.0-only

package post

import (
	"strings"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// woObject is a one-move object for exercising the work-offset substitution.
func woObject() []Object {
	return []Object{{Label: "op", Path: gcode.NewPath([]gcode.Command{gcode.NewCommand("G1", map[string]float64{"X": 1})})}}
}

// TestWorkOffsetSubstitution checks the ISO posts emit G54 by default and substitute the requested
// work coordinate system (G55) — and drop G54 — when --work-offset is given.
func TestWorkOffsetSubstitution(t *testing.T) {
	for _, post := range []string{"fanuc", "haas", "linuxcnc"} {
		def, err := Export(post, woObject(), "--no-comments")
		if err != nil {
			t.Fatalf("%s: %v", post, err)
		}
		if !strings.Contains(def, "G54") {
			t.Errorf("%s default should use G54, got:\n%s", post, def)
		}
		g55, err := Export(post, woObject(), "--no-comments --work-offset=G55")
		if err != nil {
			t.Fatalf("%s G55: %v", post, err)
		}
		if !strings.Contains(g55, "G55") || strings.Contains(g55, "G54") {
			t.Errorf("%s --work-offset=G55 should use G55 and not G54, got:\n%s", post, g55)
		}
	}
}

// TestToolLengthOffset checks the Fanuc and Haas posts activate the tool-length offset (G43 H<n>)
// right after a tool change, and that --no-tlo suppresses it.
func TestToolLengthOffset(t *testing.T) {
	withChange := []Object{{Label: "op", Path: gcode.NewPath([]gcode.Command{
		gcode.NewCommand("M6", map[string]float64{"T": 3}),
		gcode.NewCommand("G1", map[string]float64{"X": 1}),
	})}}
	for _, post := range []string{"fanuc", "haas"} {
		out, err := Export(post, withChange, "--no-comments --no-sequence-numbers")
		if err != nil {
			t.Fatalf("%s: %v", post, err)
		}
		if !strings.Contains(out, "G43 H3") {
			t.Errorf("%s should emit G43 H3 after the tool change, got:\n%s", post, out)
		}
		off, _ := Export(post, withChange, "--no-comments --no-sequence-numbers --no-tlo")
		if strings.Contains(off, "G43") {
			t.Errorf("%s --no-tlo should suppress G43, got:\n%s", post, off)
		}
	}
}

// TestSpindleStopsBeforeToolChange checks the spindle is stopped (M5) before a tool change on the
// ISO posts (before M6) and Marlin (before the manual-swap M0 pause).
func TestSpindleStopsBeforeToolChange(t *testing.T) {
	change := []Object{{Label: "op", Path: gcode.NewPath([]gcode.Command{
		gcode.NewCommand("M6", map[string]float64{"T": 1}),
		gcode.NewCommand("M3", map[string]float64{"S": 5000}),
	})}}
	for _, post := range []string{"fanuc", "haas"} {
		out, _ := Export(post, change, "--no-comments --no-sequence-numbers")
		if i, j := lineIndex(out, "M5"), lineIndex(out, "M6 T1"); i < 0 || j < 0 || i > j {
			t.Errorf("%s should stop the spindle (M5) before the M6 tool change, got:\n%s", post, out)
		}
	}
	out, _ := Export("marlin", change, "--no-comments")
	if i, j := lineIndex(out, "M5"), lineIndex(out, "M0"); i < 0 || j < 0 || i > j {
		t.Errorf("marlin should stop the spindle before the manual-swap M0, got:\n%s", out)
	}
}

// lineIndex returns the index of the first line whose trimmed text equals want, or -1.
func lineIndex(out, want string) int {
	for i, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) == want {
			return i
		}
	}
	return -1
}

// TestWorkOffsetGarbageDefaultsToG54 checks an invalid offset falls back to G54.
func TestWorkOffsetGarbageDefaultsToG54(t *testing.T) {
	out, err := Export("fanuc", woObject(), "--no-comments --work-offset=G99")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "G54") || strings.Contains(out, "G99") {
		t.Errorf("an invalid work offset should fall back to G54, got:\n%s", out)
	}
}
