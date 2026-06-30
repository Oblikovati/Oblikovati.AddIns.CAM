// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// findControl walks a control tree returning the first control matching pred.
func findControl(controls []wire.PanelControlSpec, pred func(wire.PanelControlSpec) bool) (wire.PanelControlSpec, bool) {
	for _, c := range controls {
		if pred(c) {
			return c, true
		}
		if got, ok := findControl(c.Children, pred); ok {
			return got, true
		}
	}
	return wire.PanelControlSpec{}, false
}

func hasGroupTitled(controls []wire.PanelControlSpec, title string) bool {
	_, ok := findControl(controls, func(c wire.PanelControlSpec) bool {
		return c.Kind == types.PanelGroup && c.Title == title
	})
	return ok
}

func hasButton(controls []wire.PanelControlSpec, commandID string) bool {
	_, ok := findControl(controls, func(c wire.PanelControlSpec) bool {
		return c.Kind == types.PanelButton && c.CommandID == commandID
	})
	return ok
}

// TestNewJobDialogStructure checks the New Job dialog faithfully mirrors FreeCAD's: a Template
// group, a Model group listing the document's solids as checkboxes, a Document Units group, and
// Create/Cancel buttons.
func TestNewJobDialogStructure(t *testing.T) {
	h := &recordingHost{bodies: []wire.BodyInfo{
		{Index: 0, Name: "Plate", Solid: true, Visible: true},
		{Index: 1, Name: "Bracket", Solid: true, Visible: true},
	}}
	e := NewEngine(h)
	if _, err := e.ShowNewJobDialog(); err != nil {
		t.Fatalf("ShowNewJobDialog: %v", err)
	}
	if len(h.dockWindows) == 0 {
		t.Fatal("dialog window was not set")
	}
	win := h.dockWindows[len(h.dockWindows)-1]
	if win.ID != NewJobDialogID || win.Title != "New Job" {
		t.Errorf("window id/title = %q/%q", win.ID, win.Title)
	}
	for _, group := range []string{"Template", "Model", "Document Units"} {
		if !hasGroupTitled(win.Controls, group) {
			t.Errorf("missing %q group", group)
		}
	}
	if !hasButton(win.Controls, CreateJobCommandID) || !hasButton(win.Controls, CancelNewJobCommandID) {
		t.Error("missing Create/Cancel buttons")
	}
	// One model checkbox per solid.
	count := 0
	_, _ = findControl(win.Controls, func(c wire.PanelControlSpec) bool {
		if c.Kind == types.PanelCheckBox {
			count++
		}
		return false
	})
	if count != 2 {
		t.Errorf("model checkboxes = %d, want 2 (one per solid)", count)
	}
}

// TestNewJobDialogCreatesJobFromSelection checks selecting a solid then Create builds a job whose
// model is that body.
func TestNewJobDialogCreatesJobFromSelection(t *testing.T) {
	h := &recordingHost{bodies: []wire.BodyInfo{
		{Index: 0, Name: "Plate", Solid: true, Visible: true},
		{Index: 1, Name: "Bracket", Solid: true, Visible: true},
	}}
	e := NewEngine(h)
	if _, err := e.ShowNewJobDialog(); err != nil {
		t.Fatalf("ShowNewJobDialog: %v", err)
	}
	e.applyNewJobEdit("nj_model_1", "true")
	if _, err := e.createJobAction(); err != nil {
		t.Fatalf("createJobAction: %v", err)
	}
	if e.lastJob == nil {
		t.Fatal("createJobAction did not create a job")
	}
	if len(e.lastJob.ModelBodies) != 1 || e.lastJob.ModelBodies[0] != 1 {
		t.Errorf("job model bodies = %v, want [1]", e.lastJob.ModelBodies)
	}
}
