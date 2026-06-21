// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

// TestCustomOpEmitsVerbatim checks the custom op parses its raw G-code, one command per line,
// skipping blanks, and emits the commands in order.
func TestCustomOpEmitsVerbatim(t *testing.T) {
	op := &CustomOp{OpBase: OpBase{OpLabel: "Macro", IsActive: true}, GCode: "G0 X1 Y2\n\nM8\nG1 Z-3 F100\n"}
	path, err := op.Execute(NewJob())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := commandNames(path.Commands)
	want := []string{"G0", "M8", "G1"}
	if len(got) != len(want) {
		t.Fatalf("commands = %v, want %v (blank line skipped)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("command[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	// Coordinates survive verbatim.
	if path.Commands[0].Params["X"] != 1 || path.Commands[0].Params["Y"] != 2 {
		t.Errorf("first command params = %v, want X1 Y2", path.Commands[0].Params)
	}
}

// TestCustomOpEmptyErrors checks an empty custom op is an error, not a silent no-op.
func TestCustomOpEmptyErrors(t *testing.T) {
	if _, err := (&CustomOp{OpBase: OpBase{OpLabel: "Empty"}, GCode: "  \n\n"}).Execute(NewJob()); err == nil {
		t.Error("a custom op with no G-code must error")
	}
}

// TestCustomOpRoundTrip checks the raw G-code survives a marshal/unmarshal cycle.
func TestCustomOpRoundTrip(t *testing.T) {
	job := NewJob()
	job.PostProcessor = "linuxcnc"
	job.Operations = []Operation{&CustomOp{OpBase: OpBase{OpLabel: "Probe macro", IsActive: true}, GCode: "G38.2 Z-10 F50\nG10 L20 P1 Z0"}}
	payload, err := MarshalJob(job)
	if err != nil {
		t.Fatalf("MarshalJob: %v", err)
	}
	back, err := UnmarshalJob(payload)
	if err != nil {
		t.Fatalf("UnmarshalJob: %v", err)
	}
	custom, ok := back.Operations[0].(*CustomOp)
	if !ok {
		t.Fatalf("want a *CustomOp, got %T", back.Operations[0])
	}
	if custom.GCode != "G38.2 Z-10 F50\nG10 L20 P1 Z0" || custom.OpLabel != "Probe macro" {
		t.Errorf("custom op not preserved: %+v", custom)
	}
}

// TestCustomOpParamEdit checks the G-code is editable through the parameter interface.
func TestCustomOpParamEdit(t *testing.T) {
	op := &CustomOp{}
	if !op.SetParameter("gcode", "M3 S1000") || op.GCode != "M3 S1000" {
		t.Errorf("gcode param edit failed: %q", op.GCode)
	}
}
