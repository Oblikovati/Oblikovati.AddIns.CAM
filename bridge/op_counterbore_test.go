// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestCounterboreOpExecute checks a counterbore op frames a helical recess (arcs) for each hole
// and reports the Counterbore kind.
func TestCounterboreOpExecute(t *testing.T) {
	op := &CounterboreOp{
		OpBase:   OpBase{OpLabel: "Counterbore", IsActive: true, ClearanceHeight: 15},
		Diameter: 12, Depth: 4, Pitch: 1, Holes: threadHoles(),
	}
	path, err := op.Execute(millJob(4))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	arcs := 0
	for _, c := range path.Commands {
		if c.Name == "G2" || c.Name == "G3" {
			arcs++
		}
	}
	if arcs == 0 {
		t.Error("counterbore produced no helical arcs")
	}
	if operationKind(op) != "Counterbore" {
		t.Errorf("operationKind = %q, want Counterbore", operationKind(op))
	}

	tooSmall := &CounterboreOp{OpBase: OpBase{OpLabel: "C", IsActive: true}, Diameter: 3, Depth: 4, Pitch: 1, Holes: threadHoles()}
	if _, err := tooSmall.Execute(millJob(4)); err == nil {
		t.Error("a recess no wider than the tool must error")
	}
	noHoles := &CounterboreOp{OpBase: OpBase{OpLabel: "C", IsActive: true}, Diameter: 12, Depth: 4, Pitch: 1}
	if _, err := noHoles.Execute(millJob(4)); err == nil {
		t.Error("a counterbore with no holes must error")
	}
}

// TestCounterboreParameters exercises every editable counterbore parameter.
func TestCounterboreParameters(t *testing.T) {
	op := &CounterboreOp{OpBase: OpBase{OpLabel: "Counterbore"}, Diameter: 10, Depth: 3, Pitch: 1}
	op.SetParameter("diameter", "14")
	op.SetParameter("depth", "5")
	op.SetParameter("pitch", "1.5")
	if op.Diameter != 14 || op.Depth != 5 || op.Pitch != 1.5 {
		t.Errorf("counterbore SetParameter did not apply: %+v", op)
	}
	if !op.SetParameter("label", "Spotface") {
		t.Error("name parameter should be handled by the base")
	}
	if len(op.Parameters()) < 3 {
		t.Errorf("counterbore exposes %d parameters, want at least 3", len(op.Parameters()))
	}
}

// TestCounterboreOpRoundTrip checks the counterbore op survives job serialisation.
func TestCounterboreOpRoundTrip(t *testing.T) {
	j := NewJob()
	j.PostProcessor = "grbl"
	j.Operations = []Operation{&CounterboreOp{
		OpBase: OpBase{OpLabel: "Counterbore", IsActive: true}, Diameter: 15, Depth: 6, Pitch: 1.25,
	}}
	payload, err := MarshalJob(j)
	if err != nil {
		t.Fatalf("MarshalJob: %v", err)
	}
	back, err := UnmarshalJob(payload)
	if err != nil {
		t.Fatalf("UnmarshalJob: %v", err)
	}
	op, ok := back.Operations[0].(*CounterboreOp)
	if !ok || op.Diameter != 15 || op.Depth != 6 || op.Pitch != 1.25 {
		t.Errorf("counterbore op not preserved: %+v", back.Operations[0])
	}
}
