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

// TestDuplicateOp inserts a deep copy after the selected operation and selects it.
func TestDuplicateOp(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.lastJob = twoOpJob() // [Profile, Pocket]
	e.editingOp = 0
	if _, err := e.duplicateOpAction(); err != nil {
		t.Fatalf("duplicateOpAction: %v", err)
	}
	if len(e.lastJob.Operations) != 3 {
		t.Fatalf("want 3 operations after duplicate, got %d", len(e.lastJob.Operations))
	}
	if e.lastJob.Operations[1].Label() != "Profile copy" || e.editingOp != 1 {
		t.Errorf("copy should sit at index 1 (selected): label=%q editingOp=%d", e.lastJob.Operations[1].Label(), e.editingOp)
	}
	// the copy is independent: editing its dressups must not touch the original
	orig := e.lastJob.Operations[0].(*ProfileOp)
	dup := e.lastJob.Operations[1].(*ProfileOp)
	dup.AppendDressup(NewTagsDressup(3, 2, 1))
	if len(orig.Dressups) != 0 {
		t.Error("duplicating must not share the dressup slice")
	}
}

// TestCloneEveryOp clones each operation type and checks the copy is independent + labelled.
func TestCloneEveryOp(t *testing.T) {
	ops := []Operation{
		&DrillingOp{OpBase: OpBase{OpLabel: "Drilling"}},
		&ProfileOp{OpBase: OpBase{OpLabel: "Profile"}},
		&PocketOp{OpBase: OpBase{OpLabel: "Pocket"}},
		&MillFaceOp{OpBase: OpBase{OpLabel: "Face"}},
		&EngraveOp{OpBase: OpBase{OpLabel: "Engrave"}},
		&HelixOp{OpBase: OpBase{OpLabel: "Helix"}},
		&SurfaceOp{OpBase: OpBase{OpLabel: "Surface"}},
		&WaterlineOp{OpBase: OpBase{OpLabel: "Waterline"}},
	}
	for _, op := range ops {
		clone := op.Clone()
		if clone.Label() != op.Label()+" copy" {
			t.Errorf("%T clone label = %q, want %q", op, clone.Label(), op.Label()+" copy")
		}
		clone.SetActive(true)
		if op.Active() {
			t.Errorf("%T clone shares state with the original", op)
		}
	}
}
