// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestCountersinkOpExecute checks a countersink op frames a conical spiral per hole, retracting
// between holes, and reports the Countersink kind.
func TestCountersinkOpExecute(t *testing.T) {
	op := &CountersinkOp{
		OpBase:   OpBase{OpLabel: "Countersink", IsActive: true, ClearanceHeight: 15},
		Diameter: 10, ToolAngle: 90, Holes: threadHoles(),
	}
	path, err := op.Execute(millJob(4))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !hasCutMove(path) {
		t.Error("countersink produced no cutting moves")
	}
	// two holes → two retracts to the clearance plane (z=15) among the G0 moves.
	retracts := 0
	for _, c := range path.Commands {
		if c.Name == "G0" && c.Params["Z"] == 15 {
			retracts++
		}
	}
	if retracts < 2 {
		t.Errorf("expected a retract per hole, got %d", retracts)
	}
	if operationKind(op) != "Countersink" {
		t.Errorf("operationKind = %q, want Countersink", operationKind(op))
	}

	noHoles := &CountersinkOp{OpBase: OpBase{OpLabel: "C", IsActive: true}, Diameter: 10}
	if _, err := noHoles.Execute(millJob(4)); err == nil {
		t.Error("a countersink with no holes must error")
	}
}

// TestCountersinkParameters exercises every editable countersink parameter.
func TestCountersinkParameters(t *testing.T) {
	op := &CountersinkOp{OpBase: OpBase{OpLabel: "Countersink"}, Diameter: 8, ToolAngle: 90}
	op.SetParameter("diameter", "12")
	op.SetParameter("toolAngle", "82")
	if op.Diameter != 12 || op.ToolAngle != 82 {
		t.Errorf("countersink SetParameter did not apply: %+v", op)
	}
	if !op.SetParameter("label", "CSK") {
		t.Error("name parameter should be handled by the base")
	}
	if len(op.Parameters()) < 2 {
		t.Errorf("countersink exposes %d parameters, want at least 2", len(op.Parameters()))
	}
	// Clone deep-copies and suffixes the label.
	clone, ok := op.Clone().(*CountersinkOp)
	if !ok || clone.Diameter != op.Diameter || clone.OpLabel == op.OpLabel {
		t.Errorf("Clone did not copy with a new label: %+v", clone)
	}
}

// TestCountersinkOpRoundTrip checks the countersink op survives job serialisation.
func TestCountersinkOpRoundTrip(t *testing.T) {
	j := NewJob()
	j.PostProcessor = "grbl"
	j.Operations = []Operation{&CountersinkOp{
		OpBase: OpBase{OpLabel: "Countersink", IsActive: true}, Diameter: 11, ToolAngle: 60,
	}}
	payload, err := MarshalJob(j)
	if err != nil {
		t.Fatalf("MarshalJob: %v", err)
	}
	back, err := UnmarshalJob(payload)
	if err != nil {
		t.Fatalf("UnmarshalJob: %v", err)
	}
	op, ok := back.Operations[0].(*CountersinkOp)
	if !ok || op.Diameter != 11 || op.ToolAngle != 60 {
		t.Errorf("countersink op not preserved: %+v", back.Operations[0])
	}
}
