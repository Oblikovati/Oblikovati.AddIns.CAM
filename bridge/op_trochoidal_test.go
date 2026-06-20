// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/cam/bridge/gen"
)

// TestTrochoidalOpExecute checks a trochoidal op frames a chain of loops (arcs) and reports the
// Trochoidal kind.
func TestTrochoidalOpExecute(t *testing.T) {
	op := &TrochoidalOp{
		OpBase:     OpBase{OpLabel: "Trochoidal", IsActive: true, ClearanceHeight: 15, SafeHeight: 2, StartDepth: 0, FinalDepth: -2},
		LoopRadius: 3, Advance: 4, Side: gen.SideOutside, StepDown: 3, Boundary: squarePoly(40),
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
	if arcs < 20 {
		t.Errorf("trochoidal produced %d arcs, want many overlapping loops", arcs)
	}
	if operationKind(op) != "Trochoidal" {
		t.Errorf("operationKind = %q, want Trochoidal", operationKind(op))
	}

	noBoundary := &TrochoidalOp{OpBase: OpBase{OpLabel: "T", IsActive: true}, LoopRadius: 3, Advance: 4}
	if _, err := noBoundary.Execute(millJob(4)); err == nil {
		t.Error("a trochoidal op with no boundary must error")
	}
}

// TestTrochoidalParameters exercises every editable trochoidal parameter.
func TestTrochoidalParameters(t *testing.T) {
	op := &TrochoidalOp{OpBase: OpBase{OpLabel: "Trochoidal"}, LoopRadius: 2, Advance: 2, Side: gen.SideOutside}
	op.SetParameter("loopRadius", "4")
	op.SetParameter("advance", "3")
	op.SetParameter("side", gen.SideInside)
	op.SetParameter("stepDown", "1.5")
	if op.LoopRadius != 4 || op.Advance != 3 || op.Side != gen.SideInside || op.StepDown != 1.5 {
		t.Errorf("trochoidal SetParameter did not apply: %+v", op)
	}
	if !op.SetParameter("finalDepth", "-5") {
		t.Error("depth parameter should be handled by the base")
	}
	if len(op.Parameters()) < 4 {
		t.Errorf("trochoidal exposes %d parameters, want at least 4", len(op.Parameters()))
	}
}

// TestTrochoidalOpRoundTrip checks the trochoidal op survives job serialisation.
func TestTrochoidalOpRoundTrip(t *testing.T) {
	j := NewJob()
	j.PostProcessor = "grbl"
	j.Operations = []Operation{&TrochoidalOp{
		OpBase: OpBase{OpLabel: "Trochoidal", IsActive: true}, LoopRadius: 3.5, Advance: 2.5, Side: gen.SideOutside, StepDown: 2,
	}}
	payload, err := MarshalJob(j)
	if err != nil {
		t.Fatalf("MarshalJob: %v", err)
	}
	back, err := UnmarshalJob(payload)
	if err != nil {
		t.Fatalf("UnmarshalJob: %v", err)
	}
	op, ok := back.Operations[0].(*TrochoidalOp)
	if !ok || op.LoopRadius != 3.5 || op.Advance != 2.5 || op.Side != gen.SideOutside || op.StepDown != 2 {
		t.Errorf("trochoidal op not preserved: %+v", back.Operations[0])
	}
}
