// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
)

// TestChamferOpExecute checks a chamfer op frames a single bevel pass at the angle-derived depth
// and reports the Chamfer kind.
func TestChamferOpExecute(t *testing.T) {
	op := &ChamferOp{
		OpBase: OpBase{OpLabel: "Chamfer", IsActive: true, ClearanceHeight: 15, SafeHeight: 2, StartDepth: 0},
		Width:  1.5, ToolAngle: 90, Side: gen.SideOutside, Climb: true, Boundary: squarePoly(20),
	}
	path, err := op.Execute(millJob(6))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !hasCutMove(path) {
		t.Error("chamfer produced no cutting moves")
	}
	// a 90° tool bevels to a depth equal to the width: the chamfer plunges to z = -1.5.
	if z, ok := firstPlungeZ(path); !ok || z != -1.5 {
		t.Errorf("chamfer cut depth = %g (ok=%v), want -1.5 (= width at 45° flank)", z, ok)
	}
	if operationKind(op) != "Chamfer" {
		t.Errorf("operationKind = %q, want Chamfer", operationKind(op))
	}

	noBoundary := &ChamferOp{OpBase: OpBase{OpLabel: "C", IsActive: true}, Width: 1}
	if _, err := noBoundary.Execute(millJob(6)); err == nil {
		t.Error("a chamfer with no boundary must error")
	}
}

// firstPlungeZ returns the Z of the first plunge (a G1 carrying a Z but no X/Y).
func firstPlungeZ(path gcode.Path) (float64, bool) {
	for _, c := range path.Commands {
		_, hasX := c.Params["X"]
		if z, hasZ := c.Params["Z"]; c.Name == "G1" && hasZ && !hasX {
			return z, true
		}
	}
	return 0, false
}

// TestChamferParameters exercises every editable chamfer parameter.
func TestChamferParameters(t *testing.T) {
	op := &ChamferOp{OpBase: OpBase{OpLabel: "Chamfer"}, Width: 1, ToolAngle: 90, Side: gen.SideOutside, Climb: false}
	op.SetParameter("width", "2.5")
	op.SetParameter("toolAngle", "60")
	op.SetParameter("side", gen.SideInside)
	op.SetParameter("climb", "yes")
	if op.Width != 2.5 || op.ToolAngle != 60 || op.Side != gen.SideInside || !op.Climb {
		t.Errorf("chamfer SetParameter did not apply: %+v", op)
	}
	if !op.SetParameter("label", "Deburr") { // falls through to the shared depth params
		t.Error("name parameter should be handled by the base")
	}
	if len(op.Parameters()) < 4 {
		t.Errorf("chamfer exposes %d parameters, want at least 4", len(op.Parameters()))
	}
}

// TestChamferOpRoundTrip checks the chamfer op survives job serialisation.
func TestChamferOpRoundTrip(t *testing.T) {
	j := NewJob()
	j.PostProcessor = "grbl"
	j.Operations = []Operation{&ChamferOp{
		OpBase: OpBase{OpLabel: "Chamfer", IsActive: true}, Width: 2, ToolAngle: 60, Side: gen.SideInside, Climb: true,
	}}
	payload, err := MarshalJob(j)
	if err != nil {
		t.Fatalf("MarshalJob: %v", err)
	}
	back, err := UnmarshalJob(payload)
	if err != nil {
		t.Fatalf("UnmarshalJob: %v", err)
	}
	op, ok := back.Operations[0].(*ChamferOp)
	if !ok || op.Width != 2 || op.ToolAngle != 60 || op.Side != gen.SideInside || !op.Climb {
		t.Errorf("chamfer op not preserved: %+v", back.Operations[0])
	}
}
