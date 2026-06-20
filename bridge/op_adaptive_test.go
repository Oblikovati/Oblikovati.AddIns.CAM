// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestAdaptiveOpExecute checks an adaptive op clears a region with one stay-down spiral per
// level (a single plunge) and many cutting moves.
func TestAdaptiveOpExecute(t *testing.T) {
	op := &AdaptiveOp{
		OpBase:   OpBase{OpLabel: "Adaptive", IsActive: true, ClearanceHeight: 15, SafeHeight: 2, StartDepth: 0, FinalDepth: -2},
		Climb:    true,
		Boundary: squarePoly(20),
	}
	path, err := op.Execute(millJob(4))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	plunges, cuts := 0, 0
	for _, c := range path.Commands {
		if c.Name != "G1" {
			continue
		}
		if _, hasZ := c.Params["Z"]; hasZ {
			plunges++
		}
		if _, hasX := c.Params["X"]; hasX {
			cuts++
		}
	}
	if plunges != 1 {
		t.Errorf("adaptive should stay down with one plunge per level, got %d", plunges)
	}
	if cuts < 10 {
		t.Errorf("adaptive should lay down many low-engagement passes, got %d cut moves", cuts)
	}
	if operationKind(op) != "Adaptive" {
		t.Errorf("operationKind = %q, want Adaptive", operationKind(op))
	}

	tooSmall := &AdaptiveOp{OpBase: OpBase{OpLabel: "A", IsActive: true}, Boundary: squarePoly(2)}
	if _, err := tooSmall.Execute(millJob(4)); err == nil {
		t.Error("an adaptive region smaller than the tool must error")
	}
}

// TestAdaptiveOpRoundTrip checks the adaptive op survives job serialisation with its parameters.
func TestAdaptiveOpRoundTrip(t *testing.T) {
	j := NewJob()
	j.PostProcessor = "grbl"
	j.Operations = []Operation{&AdaptiveOp{
		OpBase:   OpBase{OpLabel: "Adaptive", IsActive: true},
		StepOver: 0.15,
		Climb:    true,
		StepDown: 2.5,
	}}
	payload, err := MarshalJob(j)
	if err != nil {
		t.Fatalf("MarshalJob: %v", err)
	}
	back, err := UnmarshalJob(payload)
	if err != nil {
		t.Fatalf("UnmarshalJob: %v", err)
	}
	op, ok := back.Operations[0].(*AdaptiveOp)
	if !ok || op.StepOver != 0.15 || !op.Climb || op.StepDown != 2.5 {
		t.Errorf("adaptive op not preserved: %+v", back.Operations[0])
	}
}
