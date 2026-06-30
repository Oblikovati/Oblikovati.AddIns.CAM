// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"sort"

	"oblikovati.org/api/client"
	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// ModelSelectDialogID is the stable id of the Model Selection dialog — FreeCAD's DlgJobModelSelect,
// reached from the Job tree's Model node or the Job Edit General tab. It reuses the New Job dialog's
// solid checklist, pre-checked with the current job's model.
const ModelSelectDialogID = "com.oblikovati.cam.modelselect"

// ShowModelSelectDialog opens (or replaces) the Model Selection dialog, the document's solids with
// the current job's model pre-checked.
func (e *Engine) ShowModelSelectDialog() (wire.OKResult, error) {
	list, err := e.api.Body().List()
	if err != nil {
		return wire.OKResult{}, fmt.Errorf("list bodies: %w", err)
	}
	selected := e.resetModelSelectionLocked()
	return e.api.DockableWindows().Set(wire.DockableWindowSpec{
		ID: ModelSelectDialogID, Title: "Model Selection", Dock: types.DockFloating, Visible: true,
		Controls: []wire.PanelControlSpec{
			client.PanelGroup("ms_mdl", "Model", modelCheckList("ms", list.Bodies, selected)...),
			client.PanelGrid("ms_btns", []types.GridTrack{client.TrackFr(1), client.TrackFr(1)}, 8, 0,
				client.PanelButton("ms_apply", "Apply", ModelSelectApplyCommandID),
				client.PanelButton("ms_cancel", "Cancel", ModelSelectCancelCommandID)),
		},
	})
}

// resetModelSelectionLocked seeds the dialog's selection from the current job's model and returns a
// copy for rendering.
func (e *Engine) resetModelSelectionLocked() map[int]bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.msModel = map[int]bool{}
	if e.lastJob != nil {
		for _, idx := range e.lastJob.ModelBodies {
			e.msModel[idx] = true
		}
	}
	out := make(map[int]bool, len(e.msModel))
	for k, v := range e.msModel {
		out[k] = v
	}
	return out
}

// applyModelSelectEdit toggles one solid in the Model Selection checklist.
func (e *Engine) applyModelSelectEdit(controlID, value string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.msModel == nil {
		e.msModel = map[int]bool{}
	}
	var idx int
	if _, err := fmt.Sscanf(controlID, "ms_model_%d", &idx); err == nil {
		e.msModel[idx] = value == "true"
	}
}

// showModelSelectAction opens the Model Selection dialog (the Model node / Edit Model button).
func (e *Engine) showModelSelectAction() (*JobResult, error) {
	if _, err := e.ShowModelSelectDialog(); err != nil {
		return nil, err
	}
	return &JobResult{Summary: "CAM: Model Selection open."}, nil
}

// applyModelSelectAction writes the checked solids to the job's model, refreshes the tree, and
// dismisses the dialog.
func (e *Engine) applyModelSelectAction() (*JobResult, error) {
	e.mu.Lock()
	bodies := checkedBodies(e.msModel)
	if e.lastJob != nil {
		e.lastJob.ModelBodies = bodies
	}
	e.mu.Unlock()
	if _, err := e.api.DockableWindows().SetVisible(ModelSelectDialogID, false); err != nil {
		return nil, err
	}
	if _, err := e.ShowBrowserTree(); err != nil {
		return nil, err
	}
	return &JobResult{Summary: fmt.Sprintf("CAM: model = %d solid(s).", len(bodies))}, nil
}

// cancelModelSelectAction dismisses the Model Selection dialog without changing the model.
func (e *Engine) cancelModelSelectAction() (*JobResult, error) {
	if _, err := e.api.DockableWindows().SetVisible(ModelSelectDialogID, false); err != nil {
		return nil, err
	}
	return &JobResult{Summary: "CAM: model selection cancelled."}, nil
}

// checkedBodies returns the checked body indices of a selection map, sorted.
func checkedBodies(selection map[int]bool) []int {
	var out []int
	for idx, on := range selection {
		if on {
			out = append(out, idx)
		}
	}
	sort.Ints(out)
	return out
}
