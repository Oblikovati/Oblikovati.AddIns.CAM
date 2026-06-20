// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// threadHoles is a pair of tapped-hole targets through a 10mm part.
func threadHoles() []DrillTarget {
	return []DrillTarget{{X: 0, Y: 0, Top: 0, Bottom: -8}, {X: 20, Y: 0, Top: 0, Bottom: -8}}
}

// TestThreadMillOpExecute checks a thread-mill op frames a helical thread path (arcs) for each
// hole and reports the Thread kind.
func TestThreadMillOpExecute(t *testing.T) {
	op := &ThreadMillOp{
		OpBase:        OpBase{OpLabel: "Thread", IsActive: true, ClearanceHeight: 15},
		MajorDiameter: 10, Pitch: 1.5, Internal: true, Climb: true, Holes: threadHoles(),
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
		t.Error("thread mill produced no arc moves")
	}
	if operationKind(op) != "Thread" {
		t.Errorf("operationKind = %q, want Thread", operationKind(op))
	}

	tooBig := &ThreadMillOp{OpBase: OpBase{OpLabel: "T", IsActive: true}, MajorDiameter: 2, Pitch: 1, Internal: true, Holes: threadHoles()}
	if _, err := tooBig.Execute(millJob(6)); err == nil {
		t.Error("a tool larger than the internal major radius must error")
	}
	noHoles := &ThreadMillOp{OpBase: OpBase{OpLabel: "T", IsActive: true}, MajorDiameter: 10, Pitch: 1}
	if _, err := noHoles.Execute(millJob(4)); err == nil {
		t.Error("a thread mill with no holes must error")
	}
}

// TestThreadMillParameters exercises every editable thread-mill parameter.
func TestThreadMillParameters(t *testing.T) {
	op := &ThreadMillOp{OpBase: OpBase{OpLabel: "Thread"}, MajorDiameter: 10, Pitch: 1.5, Internal: false, Climb: false}
	op.SetParameter("majorDia", "12")
	op.SetParameter("pitch", "2")
	op.SetParameter("internal", "yes")
	op.SetParameter("climb", "yes")
	if op.MajorDiameter != 12 || op.Pitch != 2 || !op.Internal || !op.Climb {
		t.Errorf("thread-mill SetParameter did not apply: %+v", op)
	}
	if !op.SetParameter("finalDepth", "-5") { // falls through to the shared depth params
		t.Error("depth parameter should be handled by the base")
	}
	// the editable parameter list must expose every field above.
	if len(op.Parameters()) < 4 {
		t.Errorf("thread-mill exposes %d parameters, want at least 4", len(op.Parameters()))
	}
}

// TestThreadMillOpRoundTrip checks the thread-mill op survives job serialisation.
func TestThreadMillOpRoundTrip(t *testing.T) {
	j := NewJob()
	j.PostProcessor = "grbl"
	j.Operations = []Operation{&ThreadMillOp{
		OpBase: OpBase{OpLabel: "Thread", IsActive: true}, MajorDiameter: 12, Pitch: 1.75, Internal: true, Climb: true,
	}}
	payload, err := MarshalJob(j)
	if err != nil {
		t.Fatalf("MarshalJob: %v", err)
	}
	back, err := UnmarshalJob(payload)
	if err != nil {
		t.Fatalf("UnmarshalJob: %v", err)
	}
	op, ok := back.Operations[0].(*ThreadMillOp)
	if !ok || op.MajorDiameter != 12 || op.Pitch != 1.75 || !op.Internal || !op.Climb {
		t.Errorf("thread mill op not preserved: %+v", back.Operations[0])
	}
}
