// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"strings"
	"testing"

	"oblikovati.org/api/wire"
	"oblikovati.org/cam/bridge/gcode"
)

// TestRunMillFaceAndEngrave runs the new milling jobs through the recording host and checks
// they post G-code and push the two-colour toolpath preview (cuts + rapids overlays).
func TestRunMillFaceAndEngrave(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*Engine) (*JobResult, error)
		verb string
	}{
		{"face", func(e *Engine) (*JobResult, error) { return e.RunMillFaceJobOnHost(0) }, "faced"},
		{"engrave", func(e *Engine) (*JobResult, error) { return e.RunEngraveJobOnHost(0) }, "engraved"},
	} {
		h := &recordingHost{}
		res, err := tc.run(NewEngine(h))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if !strings.Contains(res.Summary, tc.verb) || res.GCodeLines == 0 {
			t.Errorf("%s summary/gcode unexpected: %q", tc.name, res.Summary)
		}
		if !h.called(wire.MethodClientGraphicsSet) {
			t.Errorf("%s should push a toolpath overlay", tc.name)
		}
		assertOverlayOnTop(t, h, tc.name)
	}
}

// assertOverlayOnTop checks every toolpath overlay primitive the engine pushed is drawn on top of
// the model, so the path is not occluded by the solid stock (the cut lies inside the part).
func assertOverlayOnTop(t *testing.T, h *recordingHost, name string) {
	t.Helper()
	if len(h.graphicsArgs) == 0 {
		t.Fatalf("%s recorded no client-graphics args", name)
	}
	for _, a := range h.graphicsArgs {
		for _, n := range a.Nodes {
			for _, p := range n.Primitives {
				if !p.OnTop {
					t.Errorf("%s overlay %q primitive is not OnTop (would be occluded by the stock)", name, a.ClientId)
				}
			}
		}
	}
}

// TestRunHelixJob bores the holes by helix and emits G2 arcs.
func TestRunHelixJob(t *testing.T) {
	res, err := NewEngine(&recordingHost{}).RunHelixJobOnHost(0)
	if err != nil {
		t.Fatalf("RunHelixJobOnHost: %v", err)
	}
	if !strings.Contains(res.GCode, "G2") && !strings.Contains(res.GCode, "G3") {
		t.Errorf("helix bore should emit arcs:\n%s", res.GCode)
	}
}

// TestShowOperationsBrowser builds the browser for a job and an empty job.
func TestShowOperationsBrowser(t *testing.T) {
	e := NewEngine(&recordingHost{})
	if _, err := e.ShowOperationsBrowser(nil); err != nil {
		t.Fatalf("empty browser: %v", err)
	}
	job := sampleJob()
	if _, err := e.ShowOperationsBrowser(job); err != nil {
		t.Fatalf("browser: %v", err)
	}
	// operationRow formatting.
	row := operationRow(0, job.Operations[0])
	if !strings.HasPrefix(row, "1. Drilling") {
		t.Errorf("row = %q, want it to start with the ordinal + kind", row)
	}
	if operationKind(&PocketOp{}) != "Pocket" || operationKind(&EngraveOp{}) != "Engrave" {
		t.Error("operationKind labels wrong")
	}
}

// TestSaveLoadActions exercises the Save/Load command actions end to end.
func TestSaveLoadActions(t *testing.T) {
	e := NewEngine(&persistHost{})
	if _, err := e.saveJobAction(); err == nil {
		t.Error("saving with no job must error")
	}
	e.lastJob = sampleJob()
	if _, err := e.saveJobAction(); err != nil {
		t.Fatalf("saveJobAction: %v", err)
	}
	res, err := e.loadJobAction()
	if err != nil {
		t.Fatalf("loadJobAction: %v", err)
	}
	if !strings.Contains(res.Summary, "loaded") {
		t.Errorf("load summary = %q", res.Summary)
	}
}

// TestToolpathPreviewSplitsMoves checks rapids and cuts land in the right groups, in cm.
func TestToolpathPreviewSplitsMoves(t *testing.T) {
	path := gcode.NewPath([]gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"X": 0, "Y": 0, "Z": 50}), // rapid (no prior pos)
		gcode.NewCommand("G0", map[string]float64{"X": 10, "Y": 0}),         // rapid
		gcode.NewCommand("G1", map[string]float64{"Z": 0}),                  // plunge (cut)
		gcode.NewCommand("G1", map[string]float64{"X": 20, "Y": 0}),         // cut
	})
	rapids, cuts := ToolpathPreview(path)
	if len(rapids.Indices) != 2 || len(cuts.Indices) != 4 {
		t.Errorf("rapids idx=%d cuts idx=%d, want 2 and 4", len(rapids.Indices), len(cuts.Indices))
	}
	// 10 mm rapid endpoint → 1 cm.
	if rapids.Coords[3] != 1.0 {
		t.Errorf("rapid endpoint X = %g cm, want 1.0 (10 mm)", rapids.Coords[3])
	}
}
