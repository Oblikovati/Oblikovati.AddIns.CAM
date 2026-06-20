// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestSlotOpExecute checks a slot op cuts a centred channel (multiple passes for a wide slot) and
// reports the Slot kind.
func TestSlotOpExecute(t *testing.T) {
	op := &SlotOp{
		OpBase: OpBase{OpLabel: "Slot", IsActive: true, ClearanceHeight: 15, SafeHeight: 2, StartDepth: 0, FinalDepth: -2},
		Width:  10, StepOver: 0.75, Climb: true, Boundary: squarePoly(40),
	}
	path, err := op.Execute(millJob(4))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if plungeCount(path) < 2 {
		t.Errorf("a 10mm slot with a 4mm tool should take several passes, got %d", plungeCount(path))
	}
	if operationKind(op) != "Slot" {
		t.Errorf("operationKind = %q, want Slot", operationKind(op))
	}

	narrow := &SlotOp{OpBase: OpBase{OpLabel: "S", IsActive: true}, Width: 2, Boundary: squarePoly(40)}
	if _, err := narrow.Execute(millJob(4)); err == nil {
		t.Error("a slot narrower than the tool must error")
	}
	noBoundary := &SlotOp{OpBase: OpBase{OpLabel: "S", IsActive: true}, Width: 10}
	if _, err := noBoundary.Execute(millJob(4)); err == nil {
		t.Error("a slot with no boundary must error")
	}
}

// TestSlotParameters exercises every editable slot parameter.
func TestSlotParameters(t *testing.T) {
	op := &SlotOp{OpBase: OpBase{OpLabel: "Slot"}, Width: 6, StepOver: 0.5, Climb: false}
	op.SetParameter("width", "9")
	op.SetParameter("stepOver", "0.7")
	op.SetParameter("stepDown", "1.5")
	op.SetParameter("climb", "yes")
	if op.Width != 9 || op.StepOver != 0.7 || op.StepDown != 1.5 || !op.Climb {
		t.Errorf("slot SetParameter did not apply: %+v", op)
	}
	if !op.SetParameter("finalDepth", "-4") {
		t.Error("depth parameter should be handled by the base")
	}
	if len(op.Parameters()) < 4 {
		t.Errorf("slot exposes %d parameters, want at least 4", len(op.Parameters()))
	}
}

// TestSlotOpRoundTrip checks the slot op survives job serialisation.
func TestSlotOpRoundTrip(t *testing.T) {
	j := NewJob()
	j.PostProcessor = "grbl"
	j.Operations = []Operation{&SlotOp{
		OpBase: OpBase{OpLabel: "Slot", IsActive: true}, Width: 8, StepOver: 0.6, Climb: true, StepDown: 2,
	}}
	payload, err := MarshalJob(j)
	if err != nil {
		t.Fatalf("MarshalJob: %v", err)
	}
	back, err := UnmarshalJob(payload)
	if err != nil {
		t.Fatalf("UnmarshalJob: %v", err)
	}
	op, ok := back.Operations[0].(*SlotOp)
	if !ok || op.Width != 8 || op.StepOver != 0.6 || !op.Climb || op.StepDown != 2 {
		t.Errorf("slot op not preserved: %+v", back.Operations[0])
	}
}
