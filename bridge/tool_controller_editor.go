// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"strconv"

	"oblikovati.org/api/client"
	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// ToolControllerEditID is the stable id of the Tool Controller editor — FreeCAD's
// DlgToolControllerEdit. It edits a tool controller's label, number, spindle and feeds; edits are
// stored as an overlay (toolEdits) applied over the derived tool list, so the feeds & speeds
// derivation and the cutter shape are preserved.
const ToolControllerEditID = "com.oblikovati.cam.tcedit"

// ShowToolControllerEditor opens (or replaces) the editor for tool controller index, seeding the
// edit overlay from its current (derived) values the first time it is opened.
func (e *Engine) ShowToolControllerEditor(index int) (wire.OKResult, error) {
	tools := e.jobTools()
	if index < 0 || index >= len(tools) {
		index = 0
	}
	e.mu.Lock()
	e.editingTool = index
	if e.toolEdits == nil {
		e.toolEdits = map[int]ToolController{}
	}
	if _, ok := e.toolEdits[index]; !ok {
		e.toolEdits[index] = tools[index]
	}
	tc := e.toolEdits[index]
	e.mu.Unlock()
	return e.api.DockableWindows().Set(wire.DockableWindowSpec{
		ID: ToolControllerEditID, Title: "Tool Controller", Dock: types.DockRight, Visible: true,
		Controls: toolEditorControls(tc),
	})
}

// toolEditorControls is the tool-controller form: identity & spindle in one group, feeds & rapids
// in another, then a Close button.
func toolEditorControls(tc ToolController) []wire.PanelControlSpec {
	return []wire.PanelControlSpec{
		camForm("tc_id", "Tool Controller",
			client.PanelTextBox("tc_label", "Label", tc.Label),
			client.PanelTextBox("tc_number", "Tool number", strconv.Itoa(tc.ToolNumber)),
			client.PanelTextBox("tc_spindle", "Spindle speed (rpm)", num(tc.SpindleSpeed)),
			client.PanelDropdown("tc_dir", "Spindle direction", []string{"Forward", "Reverse", "None"}, spindleDirOrForward(tc.SpindleDir))),
		camForm("tc_feeds", "Feeds & Rapids",
			client.PanelTextBox("tc_hfeed", "Horizontal feed (mm/min)", num(tc.HorizFeed)),
			client.PanelTextBox("tc_vfeed", "Vertical feed (mm/min)", num(tc.VertFeed)),
			client.PanelTextBox("tc_hrapid", "Horizontal rapid (mm/min)", num(tc.HorizRapid)),
			client.PanelTextBox("tc_vrapid", "Vertical rapid (mm/min)", num(tc.VertRapid)),
			client.PanelTextBox("tc_spinup", "Spin-up (s)", num(tc.SpinUpSecs))),
		client.PanelButton("tc_close", "Close", ToolEditCloseCommandID),
	}
}

// spindleDirOrForward defaults an unset spindle direction to Forward (for the dropdown).
func spindleDirOrForward(dir string) string {
	if dir == "Reverse" || dir == "None" {
		return dir
	}
	return "Forward"
}

// applyToolControllerEdit applies one tool-controller field edit to the overlay entry for the
// currently edited tool. Unknown controls / no active edit are ignored.
func (e *Engine) applyToolControllerEdit(controlID, value string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	tc, ok := e.toolEdits[e.editingTool]
	if !ok {
		return
	}
	setToolControllerField(&tc, controlID, value)
	e.toolEdits[e.editingTool] = tc
}

// setToolControllerField writes one edited field onto a tool controller, keeping the current value
// when the input is empty or malformed.
func setToolControllerField(tc *ToolController, controlID, value string) {
	switch controlID {
	case "tc_label":
		tc.Label = value
	case "tc_number":
		tc.ToolNumber = int(panelNum(value, float64(tc.ToolNumber)))
	case "tc_spindle":
		tc.SpindleSpeed = panelNum(value, tc.SpindleSpeed)
	case "tc_dir":
		if value == "Forward" || value == "Reverse" || value == "None" {
			tc.SpindleDir = value
		}
	case "tc_hfeed":
		tc.HorizFeed = panelNum(value, tc.HorizFeed)
	case "tc_vfeed":
		tc.VertFeed = panelNum(value, tc.VertFeed)
	case "tc_hrapid":
		tc.HorizRapid = panelNum(value, tc.HorizRapid)
	case "tc_vrapid":
		tc.VertRapid = panelNum(value, tc.VertRapid)
	case "tc_spinup":
		tc.SpinUpSecs = panelNum(value, tc.SpinUpSecs)
	}
}

// showToolEditAction opens the tool-controller editor for the tool the tree/Job Edit selected.
func (e *Engine) showToolEditAction() (*JobResult, error) {
	e.mu.Lock()
	idx := e.editingTool
	e.mu.Unlock()
	if _, err := e.ShowToolControllerEditor(idx); err != nil {
		return nil, err
	}
	return &JobResult{Summary: "CAM: tool controller editor open."}, nil
}

// closeToolEditAction dismisses the tool-controller editor.
func (e *Engine) closeToolEditAction() (*JobResult, error) {
	if _, err := e.api.DockableWindows().SetVisible(ToolControllerEditID, false); err != nil {
		return nil, err
	}
	return &JobResult{Summary: "CAM: tool controller editor closed."}, nil
}

// setEditingTool points the tool-controller editor at index idx (the tree's selection target).
func (e *Engine) setEditingTool(idx int) {
	e.mu.Lock()
	e.editingTool = idx
	e.mu.Unlock()
}
