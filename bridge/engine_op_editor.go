// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"strconv"
	"strings"

	"oblikovati.org/api/client"
	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// OpEditorID is the stable dockable-window id of the CAM operation editor.
const OpEditorID = "com.oblikovati.cam.opeditor"

// showOperationEditorAction opens (or refreshes) the operation editor for the last generated
// job's currently selected operation.
func (e *Engine) showOperationEditorAction() (*JobResult, error) {
	e.mu.Lock()
	job, idx := e.lastJob, e.editingOp
	e.mu.Unlock()
	if _, err := e.ShowOperationEditor(job, idx); err != nil {
		return nil, err
	}
	return &JobResult{Summary: "CAM: operation editor open."}, nil
}

// ShowOperationEditor builds the operation-editor window: a dropdown to pick which operation to
// edit, that operation's editable parameters as fields, and a Regenerate button to re-run and
// re-post the job with the edits. An empty job shows a hint. The window spec is assembled under
// the engine lock (so reading operation parameters can't race a concurrent edit), then sent.
func (e *Engine) ShowOperationEditor(job *Job, idx int) (wire.OKResult, error) {
	return e.api.DockableWindows().Set(e.editorSpec(job, idx))
}

// editorSpec builds the operation-editor window spec under the engine lock.
func (e *Engine) editorSpec(job *Job, idx int) wire.DockableWindowSpec {
	e.mu.Lock()
	defer e.mu.Unlock()
	spec := wire.DockableWindowSpec{ID: OpEditorID, Title: "CAM Operation", Dock: types.DockLeft, Visible: true}
	if job == nil || len(job.Operations) == 0 {
		spec.Controls = []wire.PanelControlSpec{client.PanelLabel("empty", "No job yet — generate one first.")}
		return spec
	}
	if idx < 0 || idx >= len(job.Operations) {
		idx = 0
	}
	labels := opChoices(job)
	op := job.Operations[idx]
	controls := []wire.PanelControlSpec{
		client.PanelLabel("hdr", "— Edit operation —"),
		client.PanelDropdown("edit_op", "Operation", labels, labels[idx]),
		client.PanelLabel("state", "Active: "+boolWord(op.Active())+dressupSummary(op)),
		client.PanelSeparator(),
	}
	controls = append(controls, opParamControls(op)...)
	spec.Controls = append(controls,
		client.PanelSeparator(),
		client.PanelButton("toggle", "Enable / Disable", ToggleOpCommandID),
		client.PanelButton("up", "Move Up", MoveOpUpCommandID),
		client.PanelButton("down", "Move Down", MoveOpDownCommandID),
		client.PanelButton("dup", "Duplicate", DuplicateOpCommandID),
		client.PanelButton("del", "Delete", DeleteOpCommandID),
		client.PanelSeparator(),
		client.PanelButton("tabs", "Add Holding Tabs", AddTabsCommandID),
		client.PanelButton("dogbone", "Add Dogbone", AddDogboneCommandID),
		client.PanelButton("ramp", "Add Ramp Entry", AddRampCommandID),
		client.PanelButton("cleardr", "Clear Dressups", ClearDressupsCommandID),
		client.PanelSeparator(),
		client.PanelButton("regen", "Regenerate + Post", RegenerateCommandID),
	)
	return spec
}

// opParamControls renders an operation's editable parameters as panel controls (or a hint when
// the operation exposes none).
func opParamControls(op Operation) []wire.PanelControlSpec {
	ed, ok := op.(Editable)
	if !ok {
		return []wire.PanelControlSpec{client.PanelLabel("noedit", "This operation has no editable parameters.")}
	}
	var controls []wire.PanelControlSpec
	for _, p := range ed.Parameters() {
		controls = append(controls, paramControl(p))
	}
	return controls
}

// paramControl maps a parameter to its panel control (dropdown for a choice, text box otherwise).
func paramControl(p OpParam) wire.PanelControlSpec {
	if p.Kind == "choice" {
		return client.PanelDropdown(p.ID, p.Label, p.Choices, p.Value)
	}
	return client.PanelTextBox(p.ID, p.Label, p.Value)
}

// dressupSummary appends a " · N dressup(s)" note when the operation carries any.
func dressupSummary(op Operation) string {
	if h, ok := op.(dressupHolder); ok && h.DressupCount() > 0 {
		return fmt.Sprintf("  ·  %d dressup(s)", h.DressupCount())
	}
	return ""
}

// opChoices labels each operation "N: Label" for the editor dropdown.
func opChoices(job *Job) []string {
	out := make([]string, len(job.Operations))
	for i, op := range job.Operations {
		out[i] = fmt.Sprintf("%d: %s", i+1, op.Label())
	}
	return out
}

// handleOpEditorEdit applies one operation-editor field change: the operation selector switches
// which operation is edited (and refreshes the window off the session goroutine), any other
// control sets a parameter on the selected operation. Runs on the host session goroutine, so it
// must not make a host call inline.
func (e *Engine) handleOpEditorEdit(controlID, value string) {
	if controlID == "edit_op" {
		e.mu.Lock()
		e.editingOp = parseOpChoice(value)
		job, idx := e.lastJob, e.editingOp
		e.mu.Unlock()
		go func() { _, _ = e.ShowOperationEditor(job, idx) }() // host call off the session goroutine
		return
	}
	e.mu.Lock()
	job, idx := e.lastJob, e.editingOp
	if job != nil && idx >= 0 && idx < len(job.Operations) {
		if ed, ok := job.Operations[idx].(Editable); ok {
			ed.SetParameter(controlID, value) // under the lock: serialised with the editor's reads
		}
	}
	e.mu.Unlock()
}

// parseOpChoice reads the leading "N:" index of an operation-dropdown value back to a 0-based
// operation index.
func parseOpChoice(value string) int {
	if i := strings.IndexByte(value, ':'); i > 0 {
		if n, err := strconv.Atoi(strings.TrimSpace(value[:i])); err == nil {
			return n - 1
		}
	}
	return 0
}

// regenerateAction re-runs the (edited) job and re-posts it, refreshing the toolpath overlay and
// G-code.
func (e *Engine) regenerateAction() (*JobResult, error) {
	e.mu.Lock()
	job := e.lastJob
	e.mu.Unlock()
	if job == nil {
		return nil, fmt.Errorf("no job to regenerate — generate one first")
	}
	return e.postPreviewResult(job, "regenerated with the edited parameters")
}
