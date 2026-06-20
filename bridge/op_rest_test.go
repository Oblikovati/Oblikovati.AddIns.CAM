// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// plungeCount counts plunge moves (G1 carrying a Z address) — one per ring per level.
func plungeCount(path gcode.Path) int {
	n := 0
	for _, c := range path.Commands {
		if _, hasZ := c.Params["Z"]; c.Name == "G1" && hasZ {
			n++
		}
	}
	return n
}

// TestRestOpExecute checks a rest op clears only the wall band (fewer rings than a full pocket of
// the same tool) and reports the Rest kind.
func TestRestOpExecute(t *testing.T) {
	base := OpBase{OpLabel: "Rest", IsActive: true, ClearanceHeight: 15, SafeHeight: 2, StartDepth: 0, FinalDepth: -2}
	rest := &RestOp{OpBase: base, PrevToolDiameter: 12, StepOver: 0.5, Climb: true, Boundary: squarePoly(20)}
	restPath, err := rest.Execute(millJob(4))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !hasCutMove(restPath) {
		t.Error("rest produced no cutting moves")
	}

	pocket := &PocketOp{OpBase: base, StepOver: 0.5, Climb: true, Boundary: squarePoly(20)}
	pocketPath, err := pocket.Execute(millJob(4))
	if err != nil {
		t.Fatalf("pocket Execute: %v", err)
	}
	if plungeCount(restPath) >= plungeCount(pocketPath) {
		t.Errorf("rest should cut fewer rings than a full pocket: rest=%d pocket=%d", plungeCount(restPath), plungeCount(pocketPath))
	}
	if operationKind(rest) != "Rest" {
		t.Errorf("operationKind = %q, want Rest", operationKind(rest))
	}

	// a previous tool no larger than the current one must error.
	bad := &RestOp{OpBase: base, PrevToolDiameter: 4, Boundary: squarePoly(20)}
	if _, err := bad.Execute(millJob(4)); err == nil {
		t.Error("a previous tool no larger than the current one must error")
	}
}

// TestRestOpRoundTrip checks the rest op survives job serialisation with its parameters.
func TestRestOpRoundTrip(t *testing.T) {
	j := NewJob()
	j.PostProcessor = "grbl"
	j.Operations = []Operation{&RestOp{
		OpBase:           OpBase{OpLabel: "Rest", IsActive: true},
		PrevToolDiameter: 10,
		StepOver:         0.4,
		Climb:            true,
		StepDown:         2,
	}}
	payload, err := MarshalJob(j)
	if err != nil {
		t.Fatalf("MarshalJob: %v", err)
	}
	back, err := UnmarshalJob(payload)
	if err != nil {
		t.Fatalf("UnmarshalJob: %v", err)
	}
	op, ok := back.Operations[0].(*RestOp)
	if !ok || op.PrevToolDiameter != 10 || op.StepOver != 0.4 || !op.Climb || op.StepDown != 2 {
		t.Errorf("rest op not preserved: %+v", back.Operations[0])
	}
}
