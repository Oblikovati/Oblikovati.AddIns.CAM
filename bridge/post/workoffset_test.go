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
