// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"oblikovati.org/api/wire"
	"oblikovati.org/cam/bridge/gcode"
)

// NewJobPath builds a Path from G-code lines, for tests.
func NewJobPath(lines ...string) gcode.Path {
	cmds := make([]gcode.Command, len(lines))
	for i, l := range lines {
		cmds[i] = gcode.ParseCommand(l)
	}
	return gcode.NewPath(cmds)
}

// commandNames returns the command names of a slice in order.
func commandNames(cmds []gcode.Command) []string {
	names := make([]string, len(cmds))
	for i, c := range cmds {
		names[i] = c.Name
	}
	return names
}

// recordingHost is a fake HostCaller that records every method called and answers the
// engine's geometry queries with a fixed two-hole plate. Unknown methods return "{}".
type recordingHost struct {
	mu      sync.Mutex
	methods []string
	failOn  string // when set, that method returns an error
}

func (h *recordingHost) Call(method string, _ []byte) ([]byte, error) {
	h.mu.Lock()
	h.methods = append(h.methods, method)
	h.mu.Unlock()
	if method == h.failOn {
		return nil, &hostError{method}
	}
	switch method {
	case wire.MethodModelReferenceKeys:
		return json.Marshal(wire.ReferenceKeysResult{Bodies: []wire.BodyTopology{{Faces: []wire.TopologyRef{
			{Kind: "plane", Point: []float64{2, 2, 1}},
			{Kind: "cylinder", Point: []float64{1, 1, 0.5}},
			{Kind: "cylinder", Point: []float64{3, 3, 0.5}},
		}}}})
	case wire.MethodBodyRangeBox:
		return json.Marshal(wire.BodyRangeBoxResult{Min: []float64{0, 0, 0}, Max: []float64{4, 4, 1}})
	case wire.MethodBrepSectionWithPlane:
		// A 4×4 cm square outline at mid-height (closed loop, cm).
		return json.Marshal(wire.BrepWiresResult{Wires: []wire.WirePolyline{{
			Points: []float64{0, 0, 0.5, 4, 0, 0.5, 4, 4, 0.5, 0, 4, 0.5, 0, 0, 0.5}, Closed: true,
		}}})
	default:
		return []byte("{}"), nil
	}
}

func (h *recordingHost) called(method string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, m := range h.methods {
		if m == method {
			return true
		}
	}
	return false
}

// waitForCall polls until the method was called or the deadline passes, for the async job
// path driven through Notify.
func (h *recordingHost) waitForCall(t *testing.T, method string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if h.called(method) {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for host method %q; got %v", method, h.methods)
}

type hostError struct{ method string }

func (e *hostError) Error() string { return "host error on " + e.method }

// TestEngineRunDrillingJob runs the full host flow and checks the queries issued, the hole
// count, and that the toolpath overlay was pushed.
func TestEngineRunDrillingJob(t *testing.T) {
	h := &recordingHost{}
	res, err := NewEngine(h).RunDrillingJobOnHost(0)
	if err != nil {
		t.Fatalf("RunDrillingJobOnHost: %v", err)
	}
	if res.HoleCount != 2 {
		t.Errorf("hole count = %d, want 2", res.HoleCount)
	}
	if res.GCodeLines == 0 || !strings.Contains(res.GCode, "G81") {
		t.Errorf("G-code looks empty / has no cycle:\n%s", res.GCode)
	}
	for _, m := range []string{wire.MethodModelReferenceKeys, wire.MethodBodyRangeBox, wire.MethodClientGraphicsSet} {
		if !h.called(m) {
			t.Errorf("expected host method %q to be called; got %v", m, h.methods)
		}
	}
}

// TestEngineGRBLPost confirms the post selection flows through to GRBL output (cycles
// translated to G0/G1, no G81 left).
func TestEngineGRBLPost(t *testing.T) {
	res, err := NewEngine(&recordingHost{}).SetPost("grbl").RunDrillingJobOnHost(0)
	if err != nil {
		t.Fatalf("RunDrillingJobOnHost: %v", err)
	}
	// No ACTIVE cycle line may remain (a commented "(G81 ...)" reference is fine).
	for _, line := range strings.Split(res.GCode, "\n") {
		if strings.HasPrefix(line, "G81") {
			t.Errorf("GRBL output left an active G81 cycle line %q:\n%s", line, res.GCode)
		}
	}
	if !strings.Contains(res.GCode, "G1 Z0.000 F100.00") {
		t.Errorf("GRBL output missing translated plunge:\n%s", res.GCode)
	}
}

// TestEngineRunProfileJob runs the contour flow: sections the body, posts a profile, and
// overlays the contour. The cut contour is the 40 mm square outline grown by the 3 mm tool
// radius.
func TestEngineRunProfileJob(t *testing.T) {
	h := &recordingHost{}
	res, err := NewEngine(h).RunProfileJobOnHost(0)
	if err != nil {
		t.Fatalf("RunProfileJobOnHost: %v", err)
	}
	if !strings.Contains(res.Summary, "profiled") || !strings.Contains(res.GCode, "G1 ") {
		t.Errorf("profile job summary/gcode unexpected: %q\n%s", res.Summary, res.GCode)
	}
	for _, m := range []string{wire.MethodBrepSectionWithPlane, wire.MethodClientGraphicsSet} {
		if !h.called(m) {
			t.Errorf("expected host method %q; got %v", m, h.methods)
		}
	}
}

// TestEngineRunPocketJob runs the area-clearing flow and checks several ring passes.
func TestEngineRunPocketJob(t *testing.T) {
	res, err := NewEngine(&recordingHost{}).SetPost("grbl").RunPocketJobOnHost(0)
	if err != nil {
		t.Fatalf("RunPocketJobOnHost: %v", err)
	}
	if !strings.Contains(res.Summary, "pocketed") {
		t.Errorf("summary = %q, want it to mention pocketed", res.Summary)
	}
	if plunges := strings.Count(res.GCode, "G1 Z"); plunges < 2 {
		t.Errorf("pocket should clear with multiple rings, got %d plunge moves", plunges)
	}
}

// TestEngineRunAdaptiveJob runs the high-speed clearing flow and checks it stays down (a single
// plunge per level), unlike the pocket flow which plunges once per ring.
func TestEngineRunAdaptiveJob(t *testing.T) {
	res, err := NewEngine(&recordingHost{}).SetPost("grbl").RunAdaptiveJobOnHost(0)
	if err != nil {
		t.Fatalf("RunAdaptiveJobOnHost: %v", err)
	}
	if !strings.Contains(res.Summary, "adaptively cleared") {
		t.Errorf("summary = %q, want it to mention adaptively cleared", res.Summary)
	}
	if cuts := strings.Count(res.GCode, "G1 X"); cuts < 10 {
		t.Errorf("adaptive should lay down many low-engagement passes, got %d cut moves", cuts)
	}
	// staying down: the pocket flow over the same body plunges far more (once per ring per level).
	pocket, err := NewEngine(&recordingHost{}).SetPost("grbl").RunPocketJobOnHost(0)
	if err != nil {
		t.Fatalf("RunPocketJobOnHost: %v", err)
	}
	if a, p := strings.Count(res.GCode, "G1 Z"), strings.Count(pocket.GCode, "G1 Z"); a >= p {
		t.Errorf("adaptive should stay down (fewer plunges) vs pocket: adaptive=%d pocket=%d", a, p)
	}
}

// TestEngineRunRestJob runs the rest-machining flow and checks it clears fewer rings than the
// full pocket flow over the same body (it only cleans the wall band).
func TestEngineRunRestJob(t *testing.T) {
	res, err := NewEngine(&recordingHost{}).SetPost("grbl").RunRestJobOnHost(0)
	if err != nil {
		t.Fatalf("RunRestJobOnHost: %v", err)
	}
	if !strings.Contains(res.Summary, "rest-cleared") {
		t.Errorf("summary = %q, want it to mention rest-cleared", res.Summary)
	}
	pocket, err := NewEngine(&recordingHost{}).SetPost("grbl").RunPocketJobOnHost(0)
	if err != nil {
		t.Fatalf("RunPocketJobOnHost: %v", err)
	}
	if r, p := strings.Count(res.GCode, "G1 Z"), strings.Count(pocket.GCode, "G1 Z"); r >= p {
		t.Errorf("rest should cut fewer rings than the full pocket: rest=%d pocket=%d", r, p)
	}
}

// TestEngineRunThreadMillJob runs the thread-milling flow and checks it threads the detected
// holes with helical arcs.
func TestEngineRunThreadMillJob(t *testing.T) {
	res, err := NewEngine(&recordingHost{}).SetPost("grbl").RunThreadMillJobOnHost(0)
	if err != nil {
		t.Fatalf("RunThreadMillJobOnHost: %v", err)
	}
	if !strings.Contains(res.Summary, "thread-milled") {
		t.Errorf("summary = %q, want it to mention thread-milled", res.Summary)
	}
	if arcs := strings.Count(res.GCode, "G3") + strings.Count(res.GCode, "G2"); arcs == 0 {
		t.Error("thread milling should emit helical arc moves")
	}
}

// TestEngineRunChamferJob runs the chamfer flow and checks it bevels the outline.
func TestEngineRunChamferJob(t *testing.T) {
	res, err := NewEngine(&recordingHost{}).SetPost("grbl").RunChamferJobOnHost(0)
	if err != nil {
		t.Fatalf("RunChamferJobOnHost: %v", err)
	}
	if !strings.Contains(res.Summary, "chamfered") {
		t.Errorf("summary = %q, want it to mention chamfered", res.Summary)
	}
	if !strings.Contains(res.GCode, "G1") {
		t.Error("chamfer should emit a cutting pass")
	}
}

// TestEngineRunTrochoidalJob runs the trochoidal flow and checks it mills the outline with arc
// loops.
func TestEngineRunTrochoidalJob(t *testing.T) {
	res, err := NewEngine(&recordingHost{}).SetPost("grbl").RunTrochoidalJobOnHost(0)
	if err != nil {
		t.Fatalf("RunTrochoidalJobOnHost: %v", err)
	}
	if !strings.Contains(res.Summary, "trochoidally milled") {
		t.Errorf("summary = %q, want it to mention trochoidally milled", res.Summary)
	}
	if arcs := strings.Count(res.GCode, "G2") + strings.Count(res.GCode, "G3"); arcs < 10 {
		t.Errorf("trochoidal should emit many loop arcs, got %d", arcs)
	}
}

// TestEngineRunSlotJob runs the slot flow and checks it cuts the centred channel.
func TestEngineRunSlotJob(t *testing.T) {
	res, err := NewEngine(&recordingHost{}).SetPost("grbl").RunSlotJobOnHost(0)
	if err != nil {
		t.Fatalf("RunSlotJobOnHost: %v", err)
	}
	if !strings.Contains(res.Summary, "slotted") {
		t.Errorf("summary = %q, want it to mention slotted", res.Summary)
	}
	if !strings.Contains(res.GCode, "G1") {
		t.Error("slot should emit cutting passes")
	}
}

// TestEngineRunProbeJob runs the probing flow and checks it emits G38.2 touch moves.
func TestEngineRunProbeJob(t *testing.T) {
	res, err := NewEngine(&recordingHost{}).SetPost("grbl").RunProbeJobOnHost(0)
	if err != nil {
		t.Fatalf("RunProbeJobOnHost: %v", err)
	}
	if !strings.Contains(res.Summary, "probed") {
		t.Errorf("summary = %q, want it to mention probed", res.Summary)
	}
	if probes := strings.Count(res.GCode, "G38.2"); probes != 3 {
		t.Errorf("probe job should emit 3 G38.2 moves, got %d", probes)
	}
}

// TestEngineRunCounterboreJob runs the counterbore flow and checks it spot-faces the holes with
// helical arcs.
func TestEngineRunCounterboreJob(t *testing.T) {
	res, err := NewEngine(&recordingHost{}).SetPost("grbl").RunCounterboreJobOnHost(0)
	if err != nil {
		t.Fatalf("RunCounterboreJobOnHost: %v", err)
	}
	if !strings.Contains(res.Summary, "counterbored") {
		t.Errorf("summary = %q, want it to mention counterbored", res.Summary)
	}
	if arcs := strings.Count(res.GCode, "G2") + strings.Count(res.GCode, "G3"); arcs == 0 {
		t.Error("counterbore should emit helical arc moves")
	}
}

// TestEngineRunCountersinkJob runs the countersink flow and checks it cuts a conical recess.
func TestEngineRunCountersinkJob(t *testing.T) {
	res, err := NewEngine(&recordingHost{}).SetPost("grbl").RunCountersinkJobOnHost(0)
	if err != nil {
		t.Fatalf("RunCountersinkJobOnHost: %v", err)
	}
	if !strings.Contains(res.Summary, "countersank") {
		t.Errorf("summary = %q, want it to mention countersank", res.Summary)
	}
	if !strings.Contains(res.GCode, "G1") {
		t.Error("countersink should emit a cutting spiral")
	}
}

// TestEngineSectionError surfaces a section failure as a job error.
func TestEngineSectionError(t *testing.T) {
	h := &recordingHost{failOn: wire.MethodBrepSectionWithPlane}
	if _, err := NewEngine(h).RunProfileJobOnHost(0); err == nil {
		t.Error("a section failure must fail the profile job")
	}
}

// TestEngineDispatchMillingCommands routes the profile/pocket command ids to their jobs.
func TestEngineDispatchMillingCommands(t *testing.T) {
	h := &recordingHost{}
	e := NewEngine(h)
	ev, _ := json.Marshal(map[string]string{"type": wire.EventCommandStarted, "command": GenerateProfileCommandID})
	e.Notify(ev)
	h.waitForCall(t, wire.MethodBrepSectionWithPlane)
}

// TestEngineReadError surfaces a host read failure as a job error.
func TestEngineReadError(t *testing.T) {
	h := &recordingHost{failOn: wire.MethodModelReferenceKeys}
	if _, err := NewEngine(h).RunDrillingJobOnHost(0); err == nil {
		t.Error("a reference-keys failure must fail the job")
	}
}

// TestEngineSetupRegistersUI checks Setup registers the command and shows the panel.
func TestEngineSetupRegistersUI(t *testing.T) {
	h := &recordingHost{}
	if err := NewEngine(h).Setup(); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if !h.called(wire.MethodCommandsCreate) || !h.called(wire.MethodDockableWindowsSet) {
		t.Errorf("Setup must register command + panel; got %v", h.methods)
	}
}

// TestNotifyPanelEdit drives a panel.valueChanged event and checks the engine state mutated
// (synchronous path; no host call).
func TestNotifyPanelEdit(t *testing.T) {
	e := NewEngine(&recordingHost{})
	ev, _ := json.Marshal(map[string]string{
		"type": wire.EventPanelValueChanged, "windowId": CAMPanelID, "controlId": "post", "value": "grbl",
	})
	e.Notify(ev)
	if e.postName != "grbl" {
		t.Errorf("post = %q, want grbl after panel edit", e.postName)
	}

	feedEv, _ := json.Marshal(map[string]string{
		"type": wire.EventPanelValueChanged, "windowId": CAMPanelID, "controlId": "plunge_feed", "value": "250 mm/min",
	})
	e.Notify(feedEv)
	if e.plungFeed != 250 {
		t.Errorf("plunge feed = %g, want 250", e.plungFeed)
	}
}

// TestApplyPanelEditIgnoresJunk confirms an unknown post or unparseable feed keeps defaults.
func TestApplyPanelEditIgnoresJunk(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.applyPanelEdit("post", "nonsense")
	e.applyPanelEdit("plunge_feed", "")
	if e.postName != "linuxcnc" || e.plungFeed != defaultPlungeFeed {
		t.Errorf("junk edits changed state: post=%q feed=%g", e.postName, e.plungFeed)
	}
}

// TestNotifyCommandStarted drives the command.started event and waits for the async job to
// run end-to-end (reaching the status report), covering launchJob + reportStatus.
func TestNotifyCommandStarted(t *testing.T) {
	h := &recordingHost{}
	e := NewEngine(h)
	ev, _ := json.Marshal(map[string]string{"type": wire.EventCommandStarted, "command": GenerateJobCommandID})
	e.Notify(ev)
	h.waitForCall(t, wire.MethodStatusSetText) // the job goroutine reports its outcome here
	if !h.called(wire.MethodModelReferenceKeys) {
		t.Errorf("the triggered job should have read geometry; got %v", h.methods)
	}
}

// TestNotifyIgnoresUnrelated covers the unmarshal-failure and unrelated-event branches.
func TestNotifyIgnoresUnrelated(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.Notify([]byte("not json"))
	other, _ := json.Marshal(map[string]string{"type": "something.else"})
	e.Notify(other) // must not panic or change state
	if e.postName != "linuxcnc" {
		t.Errorf("unrelated events must not change state")
	}
}

// TestPostObjectsInjectsToolChange checks the tool-change + spindle block is prepended.
func TestPostObjectsInjectsToolChange(t *testing.T) {
	res := []OperationResult{{
		Label:      "Drilling",
		Path:       NewJobPath("G80"),
		Controller: ToolController{ToolNumber: 4, SpindleSpeed: 1500, SpindleDir: "Reverse"},
	}}
	objs := PostObjects(res)
	if len(objs) != 1 {
		t.Fatalf("want 1 object, got %d", len(objs))
	}
	names := commandNames(objs[0].Path.Commands)
	want := []string{"M6", "M4", "G80"} // tool select, reverse spindle, then the op body
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("commands = %v, want %v", names, want)
	}
}
