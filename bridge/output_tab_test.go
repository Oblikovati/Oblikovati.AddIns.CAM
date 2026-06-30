// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"strings"
	"testing"

	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// TestOutputTabHasWcsAndOptions checks the deepened Output tab carries FreeCAD's controls: output
// file, post processor, arguments, the G54–G59 work-coordinate-system checklist, order-by and
// split-output.
func TestOutputTabHasWcsAndOptions(t *testing.T) {
	h := &recordingHost{}
	e := NewEngine(h)
	e.lastJob = &Job{}
	if _, err := e.ShowJobEditWindow(); err != nil {
		t.Fatalf("ShowJobEditWindow: %v", err)
	}
	win := h.dockWindows[len(h.dockWindows)-1]
	if _, ok := findControl(win.Controls, func(c wire.PanelControlSpec) bool {
		return c.Kind == types.PanelGroup && c.Title == "Work Coordinate Systems"
	}); !ok {
		t.Error("missing Work Coordinate Systems group")
	}
	for _, id := range []string{"out_file", "post_args", "wcs_1", "wcs_6", "order_by", "split_output"} {
		if _, ok := findControl(win.Controls, func(c wire.PanelControlSpec) bool { return c.ID == id }); !ok {
			t.Errorf("Output tab missing %q", id)
		}
	}
}

// TestOutputEditsAffectPostArgs checks the new Output fields are wired: extra arguments are
// appended and the chosen work-coordinate-system drives the post --work-offset.
func TestOutputEditsAffectPostArgs(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.applyPanelEdit("post_args", "--metric")
	e.applyPanelEdit("wcs_3", "true") // G56
	args := e.postArgs()
	if !strings.Contains(args, "--metric") {
		t.Errorf("extra arguments not appended: %q", args)
	}
	if !strings.Contains(args, "G56") {
		t.Errorf("selected WCS not reflected in args: %q", args)
	}
}
