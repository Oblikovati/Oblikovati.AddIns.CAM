// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/api/wire"
)

// TestToolEditOverridesReflectInJobTools checks the tool-controller edit store: editing a tool
// controller's fields overrides them in the job's tool list, while the base cutter (shape) is
// preserved so operations still select the tool by shape.
func TestToolEditOverridesReflectInJobTools(t *testing.T) {
	e := NewEngine(&recordingHost{})
	if _, err := e.ShowToolControllerEditor(0); err != nil { // seeds the override from the derived T1
		t.Fatalf("ShowToolControllerEditor: %v", err)
	}
	e.applyToolControllerEdit("tc_label", "Roughing mill")
	e.applyToolControllerEdit("tc_spindle", "12000")
	e.applyToolControllerEdit("tc_dir", "Reverse")

	tools := e.jobTools()
	if tools[0].Label != "Roughing mill" || tools[0].SpindleSpeed != 12000 || tools[0].SpindleDir != "Reverse" {
		t.Errorf("override not applied: %+v", tools[0])
	}
	if tools[0].Tool.ShapeType != "endmill" {
		t.Errorf("base cutter shape lost: %q", tools[0].Tool.ShapeType)
	}
}

// TestToolEditorDialogStructure checks the editor exposes the tool-controller fields.
func TestToolEditorDialogStructure(t *testing.T) {
	h := &recordingHost{}
	e := NewEngine(h)
	if _, err := e.ShowToolControllerEditor(0); err != nil {
		t.Fatalf("ShowToolControllerEditor: %v", err)
	}
	win := h.dockWindows[len(h.dockWindows)-1]
	if win.ID != ToolControllerEditID || win.Title != "Tool Controller" {
		t.Errorf("window id/title = %q/%q", win.ID, win.Title)
	}
	for _, id := range []string{"tc_label", "tc_number", "tc_spindle", "tc_dir", "tc_hfeed", "tc_vfeed"} {
		if _, ok := findControl(win.Controls, func(c wire.PanelControlSpec) bool { return c.ID == id }); !ok {
			t.Errorf("missing field %q", id)
		}
	}
}
