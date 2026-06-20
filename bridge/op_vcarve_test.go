// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestVCarveOpExecute checks a V-carve op frames nested contours deepening inward and reports the
// V-Carve kind.
func TestVCarveOpExecute(t *testing.T) {
	op := &VCarveOp{
		OpBase:    OpBase{OpLabel: "V-Carve", IsActive: true, ClearanceHeight: 15, SafeHeight: 2, StartDepth: 0},
		ToolAngle: 90, Boundary: squarePoly(40),
	}
	path, err := op.Execute(millJob(4))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var depths []float64
	for _, c := range path.Commands {
		if z, ok := c.Params["Z"]; ok && c.Name == "G1" {
			depths = append(depths, z)
		}
	}
	if len(depths) < 3 {
		t.Fatalf("V-carve should cut several nested contours, got %d", len(depths))
	}
	if depths[0] != 0 || depths[len(depths)-1] >= 0 {
		t.Errorf("V-carve should start at the surface and deepen inward, depths=%v", depths)
	}
	if operationKind(op) != "V-Carve" {
		t.Errorf("operationKind = %q, want V-Carve", operationKind(op))
	}

	noBoundary := &VCarveOp{OpBase: OpBase{OpLabel: "V", IsActive: true}, ToolAngle: 90}
	if _, err := noBoundary.Execute(millJob(4)); err == nil {
		t.Error("a V-carve with no boundary must error")
	}
}

// TestVCarveParameters exercises every editable V-carve parameter.
func TestVCarveParameters(t *testing.T) {
	op := &VCarveOp{OpBase: OpBase{OpLabel: "V-Carve"}, ToolAngle: 90, StepOver: 0.5}
	op.SetParameter("toolAngle", "60")
	op.SetParameter("stepOver", "0.3")
	if op.ToolAngle != 60 || op.StepOver != 0.3 {
		t.Errorf("V-carve SetParameter did not apply: %+v", op)
	}
	if !op.SetParameter("label", "Relief") {
		t.Error("name parameter should be handled by the base")
	}
	if len(op.Parameters()) < 2 {
		t.Errorf("V-carve exposes %d parameters, want at least 2", len(op.Parameters()))
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
		OpBase: OpBase{OpLabel: "V-Carve", IsActive: true}, ToolAngle: 60, StepOver: 0.4,
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
	if !ok || op.ToolAngle != 60 || op.StepOver != 0.4 {
		t.Errorf("V-carve op not preserved: %+v", back.Operations[0])
	}
}
