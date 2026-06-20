// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/cam/bridge/gen"
	"oblikovati.org/cam/bridge/geom2d"
)

// twoOpJob is a job with a profile and a pocket, for editor tests.
func twoOpJob() *Job {
	j := millJob(6)
	j.Operations = []Operation{
		&ProfileOp{OpBase: OpBase{OpLabel: "Profile", IsActive: true}, Side: gen.SideOutside, Boundary: squarePoly(20)},
		&PocketOp{OpBase: OpBase{OpLabel: "Pocket", IsActive: true}, StepOver: 0.5, Boundary: squarePoly(20)},
	}
	return j
}

// TestOpEditorSelectAndEdit switches the edited operation and sets one of its parameters.
func TestOpEditorSelectAndEdit(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.lastJob = twoOpJob()

	// select the second operation (Pocket) and edit its step-over
	e.handleOpEditorEdit("edit_op", "2: Pocket")
	if e.editingOp != 1 {
		t.Fatalf("editingOp = %d, want 1 after selecting '2: Pocket'", e.editingOp)
	}
	e.handleOpEditorEdit("stepOver", "0.25")
	if e.lastJob.Operations[1].(*PocketOp).StepOver != 0.25 {
		t.Errorf("pocket step-over not edited: %g", e.lastJob.Operations[1].(*PocketOp).StepOver)
	}
	// back to the first operation and edit its side
	e.handleOpEditorEdit("edit_op", "1: Profile")
	e.handleOpEditorEdit("side", gen.SideInside)
	if e.lastJob.Operations[0].(*ProfileOp).Side != gen.SideInside {
		t.Errorf("profile side not edited: %q", e.lastJob.Operations[0].(*ProfileOp).Side)
	}
}

// TestOpEditorEditNoJob is a no-op when nothing has been generated.
func TestOpEditorEditNoJob(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.handleOpEditorEdit("side", gen.SideInside) // must not panic with a nil job
}

// TestShowOperationEditor renders the editor window for a job (and the empty-job hint).
func TestShowOperationEditor(t *testing.T) {
	e := NewEngine(&recordingHost{})
	if _, err := e.ShowOperationEditor(nil, 0); err != nil {
		t.Fatalf("empty editor: %v", err)
	}
	if _, err := e.ShowOperationEditor(twoOpJob(), 0); err != nil {
		t.Fatalf("editor: %v", err)
	}
}

// TestRegenerateAction re-posts the edited job and errors with no job.
func TestRegenerateAction(t *testing.T) {
	e := NewEngine(&recordingHost{})
	if _, err := e.regenerateAction(); err == nil {
		t.Error("regenerate with no job must error")
	}
	job := millJob(6)
	job.Operations = []Operation{&ProfileOp{OpBase: OpBase{OpLabel: "Profile", IsActive: true, ClearanceHeight: 10}, Side: gen.SideOutside, Boundary: geom2d.Polygon{{X: 0, Y: 0}, {X: 20, Y: 0}, {X: 20, Y: 20}, {X: 0, Y: 20}}}}
	e.lastJob = job
	res, err := e.regenerateAction()
	if err != nil {
		t.Fatalf("regenerateAction: %v", err)
	}
	if res.GCodeLines == 0 {
		t.Error("regenerate should post G-code")
	}
}
