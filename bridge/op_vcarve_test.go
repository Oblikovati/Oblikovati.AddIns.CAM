// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestVCarveParameters exercises every editable V-carve parameter.
func TestVCarveParameters(t *testing.T) {
	op := &VCarveOp{OpBase: OpBase{OpLabel: "V-Carve"}, ToolAngle: 90, TipDiameter: 1, StepDown: 2}
	op.SetParameter("toolAngle", "60")
	op.SetParameter("tipDiameter", "0.5")
	op.SetParameter("stepDown", "3")
	if op.ToolAngle != 60 || op.TipDiameter != 0.5 || op.StepDown != 3 {
		t.Errorf("V-carve SetParameter did not apply: %+v", op)
	}
	if !op.SetParameter("label", "Relief") {
		t.Error("name parameter should be handled by the base")
	}
	if len(op.Parameters()) < 3 {
		t.Errorf("V-carve exposes %d parameters, want at least 3", len(op.Parameters()))
	}
	clone, ok := op.Clone().(*VCarveOp)
	if !ok || clone.ToolAngle != op.ToolAngle || clone.OpLabel == op.OpLabel {
		t.Errorf("Clone did not copy with a new label: %+v", clone)
	}
}

// TestVCarveOpRoundTrip checks the V-carve op survives job serialisation.
func TestVCarveOpRoundTrip(t *testing.T) {
	j := NewJob()
	j.PostProcessor = "grbl"
	j.Operations = []Operation{&VCarveOp{
		OpBase: OpBase{OpLabel: "V-Carve", IsActive: true}, ToolAngle: 60, TipDiameter: 0.5, StepDown: 2,
	}}
	payload, err := MarshalJob(j)
	if err != nil {
		t.Fatalf("MarshalJob: %v", err)
	}
	back, err := UnmarshalJob(payload)
	if err != nil {
		t.Fatalf("UnmarshalJob: %v", err)
	}
	op, ok := back.Operations[0].(*VCarveOp)
	if !ok || op.ToolAngle != 60 || op.TipDiameter != 0.5 || op.StepDown != 2 {
		t.Errorf("V-carve op not preserved: %+v", back.Operations[0])
	}
}
