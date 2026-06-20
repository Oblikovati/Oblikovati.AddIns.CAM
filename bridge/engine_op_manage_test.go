// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestToggleOp flips the selected operation's active state.
func TestToggleOp(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.lastJob = twoOpJob()
	if _, err := e.toggleOpAction(); err != nil {
		t.Fatalf("toggleOpAction: %v", err)
	}
	if e.lastJob.Operations[0].Active() {
		t.Error("first operation should be disabled after toggle")
	}
	if _, _ = e.toggleOpAction(); e.lastJob.Operations[0].Active() == false {
		t.Error("second toggle should re-enable it")
	}
}

// TestMoveOps reorders operations and keeps the selection on the moved one.
func TestMoveOps(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.lastJob = twoOpJob() // [Profile, Pocket]
	e.editingOp = 0
	if _, err := e.moveOpDownAction(); err != nil {
		t.Fatalf("moveOpDown: %v", err)
	}
	if e.lastJob.Operations[1].Label() != "Profile" || e.editingOp != 1 {
		t.Errorf("after move-down: order=%s,%s editingOp=%d", e.lastJob.Operations[0].Label(), e.lastJob.Operations[1].Label(), e.editingOp)
	}
	if _, err := e.moveOpUpAction(); err != nil {
		t.Fatalf("moveOpUp: %v", err)
	}
	if e.lastJob.Operations[0].Label() != "Profile" || e.editingOp != 0 {
		t.Errorf("after move-up: order=%s,%s editingOp=%d", e.lastJob.Operations[0].Label(), e.lastJob.Operations[1].Label(), e.editingOp)
	}
}

// TestDeleteOp removes the selected operation and clamps the selection.
func TestDeleteOp(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.lastJob = twoOpJob()
	e.editingOp = 1
	if _, err := e.deleteOpAction(); err != nil {
		t.Fatalf("deleteOpAction: %v", err)
	}
	if len(e.lastJob.Operations) != 1 || e.lastJob.Operations[0].Label() != "Profile" {
		t.Errorf("delete should leave only the profile: %+v", e.lastJob.Operations)
	}
	if e.editingOp != 0 {
		t.Errorf("editingOp should clamp to 0, got %d", e.editingOp)
	}
}

// TestMutateNoJob errors when nothing is generated.
func TestMutateNoJob(t *testing.T) {
	e := NewEngine(&recordingHost{})
	if _, err := e.toggleOpAction(); err == nil {
		t.Error("toggling with no job must error")
	}
}

// TestMoveOpBoundaries are no-ops at the ends, and the editor action opens.
func TestMoveOpBoundaries(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.lastJob = twoOpJob()
	e.editingOp = 0
	if _, err := e.moveOpUpAction(); err != nil { // already first
		t.Fatalf("moveOpUp at first: %v", err)
	}
	e.editingOp = len(e.lastJob.Operations) - 1
	if _, err := e.moveOpDownAction(); err != nil { // already last
		t.Fatalf("moveOpDown at last: %v", err)
	}
	if _, err := e.showOperationEditorAction(); err != nil {
		t.Fatalf("showOperationEditorAction: %v", err)
	}
}
