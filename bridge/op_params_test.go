// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/cam/bridge/gen"
)

// findParam returns the value of the named parameter, or "" if absent.
func findParam(params []OpParam, id string) string {
	for _, p := range params {
		if p.ID == id {
			return p.Value
		}
	}
	return ""
}

// TestProfileParameters round-trips a profile op's editable parameters.
func TestProfileParameters(t *testing.T) {
	op := &ProfileOp{OpBase: OpBase{StartDepth: 0, FinalDepth: -6}, Side: gen.SideOutside, StepDown: 3, Climb: true}
	if findParam(op.Parameters(), "side") != gen.SideOutside {
		t.Errorf("side param not surfaced: %+v", op.Parameters())
	}
	if !op.SetParameter("side", gen.SideInside) || op.Side != gen.SideInside {
		t.Errorf("set side failed: %q", op.Side)
	}
	if !op.SetParameter("stepDown", "1.5") || op.StepDown != 1.5 {
		t.Errorf("set stepDown failed: %g", op.StepDown)
	}
	if !op.SetParameter("climb", "no") || op.Climb {
		t.Errorf("set climb failed: %v", op.Climb)
	}
	if !op.SetParameter("finalDepth", "-8") || op.FinalDepth != -8 { // common depth param
		t.Errorf("set finalDepth failed: %g", op.FinalDepth)
	}
	if op.SetParameter("nonexistent", "1") {
		t.Error("setting an unknown parameter must report false")
	}
}

// TestDrillingParameters covers numeric + bool drilling params.
func TestDrillingParameters(t *testing.T) {
	op := &DrillingOp{OpBase: OpBase{IsActive: true}, PeckDepth: 1, Repeat: 1}
	if !op.SetParameter("peckDepth", "2.5") || op.PeckDepth != 2.5 {
		t.Errorf("peck depth: %g", op.PeckDepth)
	}
	if !op.SetParameter("repeat", "3") || op.Repeat != 3 {
		t.Errorf("repeat: %d", op.Repeat)
	}
	if !op.SetParameter("chipBreak", "yes") || !op.ChipBreak {
		t.Errorf("chipBreak: %v", op.ChipBreak)
	}
}

// TestEveryOpIsEditable confirms each concrete operation exposes editable parameters.
func TestEveryOpIsEditable(t *testing.T) {
	ops := []Operation{
		&DrillingOp{}, &ProfileOp{}, &PocketOp{}, &AdaptiveOp{}, &RestOp{}, &TrochoidalOp{}, &SlotOp{}, &MillFaceOp{}, &EngraveOp{}, &ChamferOp{}, &HelixOp{}, &ThreadMillOp{}, &CounterboreOp{}, &CountersinkOp{}, &ProbeOp{}, &SurfaceOp{}, &WaterlineOp{},
	}
	for _, op := range ops {
		ed, ok := op.(Editable)
		if !ok {
			t.Errorf("%T is not Editable", op)
			continue
		}
		if len(ed.Parameters()) == 0 {
			t.Errorf("%T exposes no parameters", op)
		}
	}
}

// TestParseOpChoice reads the operation index back from a dropdown label.
func TestParseOpChoice(t *testing.T) {
	if parseOpChoice("2: Profile") != 1 {
		t.Error("parseOpChoice should map '2: Profile' to index 1")
	}
	if parseOpChoice("garbage") != 0 {
		t.Error("parseOpChoice should default to 0")
	}
}

// TestRenameOperation edits an operation's name through the parameter interface.
func TestRenameOperation(t *testing.T) {
	op := &ProfileOp{OpBase: OpBase{OpLabel: "Profile"}}
	if findParam(op.Parameters(), "label") != "Profile" {
		t.Errorf("name param not surfaced: %+v", op.Parameters())
	}
	if !op.SetParameter("label", "Roughing") || op.Label() != "Roughing" {
		t.Errorf("rename failed: %q", op.Label())
	}
	if op.SetParameter("label", ""); op.Label() != "Roughing" {
		t.Errorf("an empty name must be ignored, got %q", op.Label())
	}
}
