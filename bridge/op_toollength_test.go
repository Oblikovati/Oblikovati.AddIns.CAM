// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestToolLengthProbeOpExecute checks the op frames a single G38.2 probe + a G10 L1 tool-length
// set, cuts nothing, and reports the Tool Probe kind.
func TestToolLengthProbeOpExecute(t *testing.T) {
	op := &ToolLengthProbeOp{
		OpBase:  OpBase{OpLabel: "Tool Probe", IsActive: true, ClearanceHeight: 60},
		SetterX: -50, SetterY: -50, SetterTop: 25, ToolNumber: 2, ProbeFeed: 40,
	}
	path, err := op.Execute(NewJob())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	probes, sets := 0, 0
	for _, c := range path.Commands {
		switch c.Name {
		case "G38.2":
			probes++
		case "G10":
			sets++
			if c.Params["L"] != 1 || c.Params["P"] != 2 {
				t.Errorf("expected a G10 L1 P2 tool-length set, got %+v", c.Params)
			}
		case "G1":
			t.Errorf("a tool-length probe should cut nothing, found a G1: %+v", c.Params)
		}
	}
	if probes != 1 || sets != 1 {
		t.Errorf("want one probe + one set, got probes=%d sets=%d", probes, sets)
	}
	if operationKind(op) != "Tool Probe" {
		t.Errorf("operationKind = %q, want Tool Probe", operationKind(op))
	}

	noTool := &ToolLengthProbeOp{OpBase: OpBase{OpLabel: "T", IsActive: true}, ProbeFeed: 40}
	if _, err := noTool.Execute(NewJob()); err == nil {
		t.Error("a tool-length probe with no tool number must error")
	}
}

// TestToolLengthProbeParameters exercises every editable parameter and the Clone.
func TestToolLengthProbeParameters(t *testing.T) {
	op := &ToolLengthProbeOp{OpBase: OpBase{OpLabel: "Tool Probe"}, SetterX: -50, SetterY: -50, SetterTop: 25, ToolNumber: 1}
	op.SetParameter("setterX", "-40")
	op.SetParameter("setterY", "-60")
	op.SetParameter("setterTop", "30")
	op.SetParameter("toolNumber", "4")
	op.SetParameter("probeFeed", "35")
	if op.SetterX != -40 || op.SetterY != -60 || op.SetterTop != 30 || op.ToolNumber != 4 || op.ProbeFeed != 35 {
		t.Errorf("tool-length SetParameter did not apply: %+v", op)
	}
	if op.feedRate() != 35 {
		t.Errorf("feedRate = %g, want 35", op.feedRate())
	}
	if !op.SetParameter("label", "TLO") { // falls through to the shared depth params
		t.Error("name parameter should be handled by the base")
	}
	if len(op.Parameters()) < 5 {
		t.Errorf("tool-length probe exposes %d parameters, want at least 5", len(op.Parameters()))
	}
	clone, ok := op.Clone().(*ToolLengthProbeOp)
	if !ok || clone.SetterTop != op.SetterTop || clone.OpLabel == op.OpLabel {
		t.Errorf("Clone did not copy with a new label: %+v", clone)
	}
}

// TestToolLengthProbeOpRoundTrip checks the op survives job serialisation.
func TestToolLengthProbeOpRoundTrip(t *testing.T) {
	j := NewJob()
	j.PostProcessor = "grbl"
	j.Operations = []Operation{&ToolLengthProbeOp{
		OpBase: OpBase{OpLabel: "Tool Probe", IsActive: true}, SetterX: -40, SetterY: -60, SetterTop: 30, ToolNumber: 5, ProbeFeed: 35,
	}}
	payload, err := MarshalJob(j)
	if err != nil {
		t.Fatalf("MarshalJob: %v", err)
	}
	back, err := UnmarshalJob(payload)
	if err != nil {
		t.Fatalf("UnmarshalJob: %v", err)
	}
	op, ok := back.Operations[0].(*ToolLengthProbeOp)
	if !ok || op.SetterX != -40 || op.SetterY != -60 || op.SetterTop != 30 || op.ToolNumber != 5 || op.ProbeFeed != 35 {
		t.Errorf("tool-length probe op not preserved: %+v", back.Operations[0])
	}
}
