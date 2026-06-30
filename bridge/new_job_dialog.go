// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"strings"

	"oblikovati.org/api/client"
	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// NewJobDialogID is the stable dockable-window id of the New Job dialog. Faithful to FreeCAD's
// New Job dialog (Template / Model / Document Units / OK-Cancel); built with the grid+group
// layout vocabulary. Per ADR-0021 it is an interim non-modal panel with Create/Cancel buttons.
const NewJobDialogID = "com.oblikovati.cam.newjob"

// jobTemplateOptions are the job templates offered in the New Job dialog. Milestone-1 ships the
// built-in starting points; user templates come later.
func jobTemplateOptions() []string {
	return []string{"(none)", "Default (mm)", "Default (inch)"}
}

// unitSchemaOptions are the document-unit schemas; the per-minute ones are recommended for safe
// G-code feed rates (FreeCAD colours the per-second ones red).
func unitSchemaOptions() []string {
	return []string{"mm/min (recommended)", "in/min", "mm/s (unsafe)", "in/s (unsafe)"}
}

// ShowNewJobDialog opens (or replaces) the New Job dialog, listing the document's solids as a
// checklist of model candidates alongside the template and unit choices.
func (e *Engine) ShowNewJobDialog() (wire.OKResult, error) {
	list, err := e.api.Body().List()
	if err != nil {
		return wire.OKResult{}, fmt.Errorf("list bodies: %w", err)
	}
	e.mu.Lock()
	tpl, units, selected := e.njDefaultsLocked()
	e.mu.Unlock()
	return e.api.DockableWindows().Set(wire.DockableWindowSpec{
		ID: NewJobDialogID, Title: "New Job", Dock: types.DockFloating, Visible: true,
		Controls: newJobControls(list.Bodies, tpl, units, selected),
	})
}

// njDefaultsLocked returns the current dialog selections, filling sane defaults the first time.
// The caller must hold e.mu.
func (e *Engine) njDefaultsLocked() (template, units string, selected map[int]bool) {
	if e.njTemplate == "" {
		e.njTemplate = jobTemplateOptions()[0]
	}
	if e.njUnits == "" {
		e.njUnits = unitSchemaOptions()[0]
	}
	if e.njModel == nil {
		e.njModel = map[int]bool{}
	}
	selected = make(map[int]bool, len(e.njModel))
	for k, v := range e.njModel {
		selected[k] = v
	}
	return e.njTemplate, e.njUnits, selected
}

// newJobControls builds the dialog body: Template, Model (one checkbox per solid), Document
// Units, and a Create/Cancel button row.
func newJobControls(bodies []wire.BodyInfo, template, units string, selected map[int]bool) []wire.PanelControlSpec {
	return []wire.PanelControlSpec{
		client.PanelGroup("nj_tpl", "Template",
			client.PanelDropdown("nj_template", "Template", jobTemplateOptions(), template)),
		client.PanelGroup("nj_mdl", "Model", modelCheckList("nj", bodies, selected)...),
		client.PanelGroup("nj_unt", "Document Units",
			client.PanelDropdown("nj_units", "Units", unitSchemaOptions(), units)),
		client.PanelGrid("nj_btns", []types.GridTrack{client.TrackFr(1), client.TrackFr(1)}, 8, 0,
			client.PanelButton("nj_create", "Create Job", CreateJobCommandID),
			client.PanelButton("nj_cancel", "Cancel", CancelNewJobCommandID)),
	}
}

// modelCheckList is one checkbox per solid body (FreeCAD's Solids list), or a placeholder when
// the document has no solids to machine. idPrefix namespaces the checkbox ids ("<prefix>_model_N")
// so the New Job dialog and the Model Selection dialog can share this builder.
func modelCheckList(idPrefix string, bodies []wire.BodyInfo, selected map[int]bool) []wire.PanelControlSpec {
	var controls []wire.PanelControlSpec
	for _, b := range bodies {
		if !b.Solid {
			continue
		}
		controls = append(controls, client.PanelCheckBox(
			fmt.Sprintf("%s_model_%d", idPrefix, b.Index), bodyCheckLabel(b), selected[b.Index]))
	}
	if len(controls) == 0 {
		controls = append(controls, client.PanelLabel(idPrefix+"_nomodel", "(no solids in this document)"))
	}
	return controls
}

// bodyCheckLabel labels a model checkbox with the body's name (falling back to its index).
func bodyCheckLabel(b wire.BodyInfo) string {
	if strings.TrimSpace(b.Name) != "" {
		return b.Name
	}
	return fmt.Sprintf("Body %d", b.Index)
}

// applyNewJobEdit records one New Job dialog edit: the template, the units, or a model checkbox.
func (e *Engine) applyNewJobEdit(controlID, value string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.njModel == nil {
		e.njModel = map[int]bool{}
	}
	switch {
	case controlID == "nj_template":
		e.njTemplate = value
	case controlID == "nj_units":
		e.njUnits = value
	case strings.HasPrefix(controlID, "nj_model_"):
		var idx int
		if _, err := fmt.Sscanf(controlID, "nj_model_%d", &idx); err == nil {
			e.njModel[idx] = value == "true"
		}
	}
}

// newJobDialogAction opens the New Job dialog (the New Job command).
func (e *Engine) newJobDialogAction() (*JobResult, error) {
	if _, err := e.ShowNewJobDialog(); err != nil {
		return nil, err
	}
	return &JobResult{Summary: "CAM: New Job dialog open."}, nil
}

// createJobAction creates an empty job from the dialog selections (its model bodies), stores it
// as the current job, dismisses the dialog, and reports what was created. Operations are added
// afterwards from the CAM tab — this mirrors FreeCAD, where New Job yields an empty job.
func (e *Engine) createJobAction() (*JobResult, error) {
	e.mu.Lock()
	bodies := e.selectedModelLocked()
	e.mu.Unlock()

	job := NewJob()
	job.ModelBodies = bodies
	job.Tools = e.jobTools() // jobTools locks e.mu itself, so it must run OUTSIDE the section above

	e.mu.Lock()
	e.lastJob = job
	e.targetBody = bodies[0]
	e.mu.Unlock()

	if _, err := e.api.DockableWindows().SetVisible(NewJobDialogID, false); err != nil {
		return nil, err
	}
	if _, err := e.ShowBrowserTree(); err != nil { // surface the new Job in the model browser, like FreeCAD
		return nil, err
	}
	return &JobResult{Summary: fmt.Sprintf("CAM: created job over %d model solid(s).", len(bodies))}, nil
}

// selectedModelLocked returns the checked body indices (sorted), defaulting to the current target
// body when nothing is checked so a job always has a model. The caller must hold e.mu.
func (e *Engine) selectedModelLocked() []int {
	bodies := checkedBodies(e.njModel)
	if len(bodies) == 0 {
		return []int{e.targetBody} // a job always has a model
	}
	return bodies
}

// cancelNewJobAction dismisses the New Job dialog without creating a job.
func (e *Engine) cancelNewJobAction() (*JobResult, error) {
	if _, err := e.api.DockableWindows().SetVisible(NewJobDialogID, false); err != nil {
		return nil, err
	}
	return &JobResult{Summary: "CAM: New Job cancelled."}, nil
}
