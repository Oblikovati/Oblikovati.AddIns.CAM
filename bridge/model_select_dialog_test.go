// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/api/wire"
)

// TestModelSelectDialogPrechecksJobModel checks the Model Selection dialog lists the document's
// solids with the current job's model pre-checked, plus Apply/Cancel.
func TestModelSelectDialogPrechecksJobModel(t *testing.T) {
	h := &recordingHost{bodies: []wire.BodyInfo{
		{Index: 0, Name: "A", Solid: true, Visible: true},
		{Index: 1, Name: "B", Solid: true, Visible: true},
	}}
	e := NewEngine(h)
	e.lastJob = &Job{ModelBodies: []int{1}}

	if _, err := e.ShowModelSelectDialog(); err != nil {
		t.Fatalf("ShowModelSelectDialog: %v", err)
	}
	win := h.dockWindows[len(h.dockWindows)-1]
	if win.ID != ModelSelectDialogID || win.Title != "Model Selection" {
		t.Errorf("window id/title = %q/%q", win.ID, win.Title)
	}
	one, ok := findControl(win.Controls, func(c wire.PanelControlSpec) bool { return c.ID == "ms_model_1" })
	if !ok || one.Value != "true" {
		t.Errorf("body 1 should be pre-checked, got %+v", one)
	}
	zero, _ := findControl(win.Controls, func(c wire.PanelControlSpec) bool { return c.ID == "ms_model_0" })
	if zero.Value == "true" {
		t.Error("body 0 should be unchecked")
	}
	if !hasButton(win.Controls, ModelSelectApplyCommandID) || !hasButton(win.Controls, ModelSelectCancelCommandID) {
		t.Error("missing Apply/Cancel buttons")
	}
}

// TestModelSelectApplyUpdatesJobModel checks toggling the checklist then Apply rewrites the job's
// model bodies.
func TestModelSelectApplyUpdatesJobModel(t *testing.T) {
	h := &recordingHost{bodies: []wire.BodyInfo{
		{Index: 0, Name: "A", Solid: true, Visible: true},
		{Index: 1, Name: "B", Solid: true, Visible: true},
	}}
	e := NewEngine(h)
	e.lastJob = &Job{ModelBodies: []int{0}}

	if _, err := e.ShowModelSelectDialog(); err != nil {
		t.Fatalf("ShowModelSelectDialog: %v", err)
	}
	e.applyModelSelectEdit("ms_model_0", "false")
	e.applyModelSelectEdit("ms_model_1", "true")
	if _, err := e.applyModelSelectAction(); err != nil {
		t.Fatalf("applyModelSelectAction: %v", err)
	}
	if len(e.lastJob.ModelBodies) != 1 || e.lastJob.ModelBodies[0] != 1 {
		t.Errorf("job model = %v, want [1]", e.lastJob.ModelBodies)
	}
}
