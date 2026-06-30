// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"oblikovati.org/api/types"
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
	mu            sync.Mutex
	methods       []string
	failOn        string              // when set, that method returns an error
	sectionWires  []wire.WirePolyline // when set, overrides the default single-square section
	graphicsArgs  []wire.SetClientGraphicsArgs
	createdCmds   []wire.CreateCommandArgs // every commands.create request, for ribbon-placement assertions
	minDistanceCm float64                  // body.minimumDistance reply (cm); 0 (default) blocks every keep-down link
}

func (h *recordingHost) Call(method string, payload []byte) ([]byte, error) {
	h.mu.Lock()
	h.methods = append(h.methods, method)
	if method == wire.MethodClientGraphicsSet {
		var a wire.SetClientGraphicsArgs
		if json.Unmarshal(payload, &a) == nil {
			h.graphicsArgs = append(h.graphicsArgs, a)
		}
	}
	if method == wire.MethodCommandsCreate {
		var a wire.CreateCommandArgs
		if json.Unmarshal(payload, &a) == nil {
			h.createdCmds = append(h.createdCmds, a)
		}
	}
	h.mu.Unlock()
	if method == h.failOn {
		return nil, &hostError{method}
	}
	switch method {
	case wire.MethodModelReferenceKeys:
		return json.Marshal(wire.ReferenceKeysResult{Bodies: []wire.BodyTopology{{Faces: []wire.TopologyRef{
			{Key: "f0", Kind: "plane", Point: []float64{2, 2, 1}},
			{Key: "f1", Kind: "cylinder", Point: []float64{1, 1, 0.5}},
			{Key: "f2", Kind: "cylinder", Point: []float64{3, 3, 0.5}},
		}}}})
	case wire.MethodBrepOffsetFaces:
		return json.Marshal(wire.BrepHandleResult{Handle: 11, Stats: wire.BrepBodyStats{Faces: 3}})
	case wire.MethodBodyRangeBox:
		return json.Marshal(wire.BodyRangeBoxResult{Min: []float64{0, 0, 0}, Max: []float64{4, 4, 1}})
	case wire.MethodBodyMinimumDistance:
		return json.Marshal(wire.MinimumDistanceResult{Distance: h.minDistanceCm})
	case wire.MethodBrepSectionWithPlane:
		if h.sectionWires != nil {
			return json.Marshal(wire.BrepWiresResult{Wires: h.sectionWires})
		}
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

// TestEngineRunBoreProbeJob runs the bore-centre probing flow and checks it emits four G38.2
// touch moves per detected hole.
func TestEngineRunBoreProbeJob(t *testing.T) {
	res, err := NewEngine(&recordingHost{}).SetPost("grbl").RunBoreProbeJobOnHost(0)
	if err != nil {
		t.Fatalf("RunBoreProbeJobOnHost: %v", err)
	}
	if !strings.Contains(res.Summary, "bore-probed") {
		t.Errorf("summary = %q, want it to mention bore-probed", res.Summary)
	}
	if probes := strings.Count(res.GCode, "G38.2"); probes == 0 || probes%4 != 0 {
		t.Errorf("bore probing should emit four G38.2 moves per hole, got %d", probes)
	}
}

// TestEngineRunToolProbeJob runs the tool-length probing flow and checks it emits one G38.2 probe
// and a G10 L1 tool-length set.
func TestEngineRunToolProbeJob(t *testing.T) {
	res, err := NewEngine(&recordingHost{}).SetPost("grbl").RunToolLengthProbeJobOnHost(0)
	if err != nil {
		t.Fatalf("RunToolLengthProbeJobOnHost: %v", err)
	}
	if !strings.Contains(res.Summary, "tool length") {
		t.Errorf("summary = %q, want it to mention the tool length", res.Summary)
	}
	if probes := strings.Count(res.GCode, "G38.2"); probes != 1 {
		t.Errorf("tool-length probing should emit one G38.2, got %d", probes)
	}
	if !strings.Contains(res.GCode, "G10") || !strings.Contains(res.GCode, "L1") {
		t.Errorf("tool-length probing should set a G10 L1 offset:\n%s", res.GCode)
	}
}

// TestEngineRunBossProbeJob runs the boss-centre probing flow and checks it emits four inward
// G38.2 touch moves (and sets no work offset — the centre comes from averaging).
func TestEngineRunBossProbeJob(t *testing.T) {
	res, err := NewEngine(&recordingHost{}).SetPost("grbl").RunBossProbeJobOnHost(0)
	if err != nil {
		t.Fatalf("RunBossProbeJobOnHost: %v", err)
	}
	if !strings.Contains(res.Summary, "boss-probed") {
		t.Errorf("summary = %q, want it to mention boss-probed", res.Summary)
	}
	if probes := strings.Count(res.GCode, "G38.2"); probes != 4 {
		t.Errorf("boss probing should emit four G38.2 moves, got %d", probes)
	}
	if strings.Contains(res.GCode, "G10") {
		t.Errorf("boss probing should not set a work offset (centre comes from averaging):\n%s", res.GCode)
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
	// the corner cycle zeroes all three axes of the work offset (one G10 per touch).
	if sets := strings.Count(res.GCode, "G10"); sets != 3 {
		t.Errorf("corner probe should set the work offset on all three axes, got %d G10s", sets)
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

// TestEngineRunTappingJobISO runs the tapping flow on an ISO controller and checks it emits a
// native G84 tap cycle and cancels it with G80.
func TestEngineRunTappingJobISO(t *testing.T) {
	res, err := NewEngine(&recordingHost{}).SetPost("linuxcnc").RunTappingJobOnHost(0)
	if err != nil {
		t.Fatalf("RunTappingJobOnHost: %v", err)
	}
	if !strings.Contains(res.Summary, "tapped") {
		t.Errorf("summary = %q, want it to mention tapped", res.Summary)
	}
	if !strings.Contains(res.GCode, "G84") {
		t.Error("an ISO post should emit a native G84 tap cycle")
	}
	if !strings.Contains(res.GCode, "G80") {
		t.Error("the tap cycle should be cancelled with G80")
	}
	// Tapping runs in feed-per-revolution: G95 before the cycle (so F is the pitch per turn) and
	// G94 restored after — the feed-per-rev fidelity the reference workbench relies on.
	if !strings.Contains(res.GCode, "G95") {
		t.Error("tapping should switch to feed-per-revolution (G95)")
	}
	if !strings.Contains(res.GCode, "G94") {
		t.Error("tapping should restore feed-per-minute (G94) after the cycle")
	}
}

// TestEngineRunTappingJobGRBL checks GRBL — which lacks rigid tapping — expands the cycle into
// explicit feed moves rather than leaking a G84 code it cannot run.
func TestEngineRunTappingJobGRBL(t *testing.T) {
	res, err := NewEngine(&recordingHost{}).SetPost("grbl").RunTappingJobOnHost(0)
	if err != nil {
		t.Fatalf("RunTappingJobOnHost: %v", err)
	}
	for _, line := range strings.Split(res.GCode, "\n") {
		code := strings.TrimSpace(line)
		if strings.HasPrefix(code, "G84") || strings.HasPrefix(code, "G74") {
			t.Errorf("GRBL has no rigid tapping; the cycle must be expanded, not emitted as an active code: %q", line)
		}
		// GRBL has no feed-per-rev mode: the op's G95/G94 must be commented out, not left active.
		if strings.HasPrefix(code, "G95") || strings.HasPrefix(code, "G94") {
			t.Errorf("GRBL cannot run feed-per-rev mode; G95/G94 must be commented out, not active: %q", line)
		}
	}
	if !strings.Contains(res.GCode, "soft tap") {
		t.Error("the GRBL expansion should flag the soft tap in a comment")
	}
	if !strings.Contains(res.GCode, "G1 Z") {
		t.Error("the GRBL tap expansion should feed in and out with G1 Z moves")
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

// TestPanelMaterialFeeds checks selecting a material sets the active end mill's spindle speed and
// feed from the feeds & speeds calculator, and that changing the tool diameter re-derives them.
func TestPanelMaterialFeeds(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.applyPanelEdit("tool_dia", "6")
	e.applyPanelEdit("material", "aluminium")
	tc := e.activeEndmill()
	if tc.SpindleSpeed <= 5000 {
		t.Errorf("aluminium @6mm should drive the spindle well above the 5000 default, got %g", tc.SpindleSpeed)
	}
	if tc.HorizFeed <= 0 {
		t.Errorf("material selection should set a cutting feed, got %g", tc.HorizFeed)
	}

	// a harder material drops the RPM; switching to steel must slow the spindle.
	e.applyPanelEdit("material", "steel")
	if steel := e.activeEndmill(); steel.SpindleSpeed >= tc.SpindleSpeed {
		t.Errorf("steel RPM %g should be below aluminium %g", steel.SpindleSpeed, tc.SpindleSpeed)
	}

	// an unknown material leaves the feeds unchanged.
	before := e.activeEndmill().SpindleSpeed
	e.applyPanelEdit("material", "unobtanium")
	if after := e.activeEndmill().SpindleSpeed; after != before {
		t.Errorf("unknown material should not change the spindle (%g → %g)", before, after)
	}

	// more flutes → a faster feed at the same RPM (feed = RPM · flutes · chipload).
	e.applyPanelEdit("material", "aluminium")
	two := e.activeEndmill()
	e.applyPanelEdit("flutes", "4")
	four := e.activeEndmill()
	if four.Tool.Flutes != 4 {
		t.Errorf("flute count not applied to the tool: %d", four.Tool.Flutes)
	}
	if four.SpindleSpeed != two.SpindleSpeed {
		t.Errorf("flute count should not change the RPM (%g → %g)", two.SpindleSpeed, four.SpindleSpeed)
	}
	if four.HorizFeed <= two.HorizFeed {
		t.Errorf("4 flutes should feed faster than 2 (%g vs %g)", four.HorizFeed, two.HorizFeed)
	}
}

// TestEngineHaasPost checks the panel can select the Haas post and a job posts in its dialect
// (five-digit O-number + safe-start block + G28 home return).
func TestEngineHaasPost(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.applyPanelEdit("post", "haas")
	res, err := e.RunProfileJobOnHost(0)
	if err != nil {
		t.Fatalf("RunProfileJobOnHost: %v", err)
	}
	if !strings.HasPrefix(res.GCode, "%\n") || !strings.Contains(res.GCode, "O00001") || !strings.Contains(res.GCode, "G40 G49 G80") || !strings.Contains(res.GCode, "G28 G91 Z0.") {
		t.Errorf("expected Haas-dialect output (O00001 / safe-start / G28 home):\n%s", res.GCode)
	}
}

// TestEngineMarlinPost checks the panel can select the Marlin post and a job posts in its dialect
// (semicolon comments + metric/absolute preamble, no tape wrapper).
func TestEngineMarlinPost(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.applyPanelEdit("post", "marlin")
	res, err := e.RunProfileJobOnHost(0)
	if err != nil {
		t.Fatalf("RunProfileJobOnHost: %v", err)
	}
	if !strings.HasPrefix(res.GCode, "; Exported by Oblikovati\n") || !strings.Contains(res.GCode, "\nG90\n") || !strings.Contains(res.GCode, "\nM5\n") {
		t.Errorf("expected Marlin-dialect output (; comments / G90 / M5):\n%s", res.GCode)
	}
}

// TestEngineFanucPost checks the panel can select the Fanuc post and a job posts in its dialect
// (tape wrapper + O-number program header).
func TestEngineFanucPost(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.applyPanelEdit("post", "fanuc")
	res, err := e.RunProfileJobOnHost(0)
	if err != nil {
		t.Fatalf("RunProfileJobOnHost: %v", err)
	}
	if !strings.HasPrefix(res.GCode, "%\n") || !strings.Contains(res.GCode, "O0001") || !strings.Contains(res.GCode, "M30") {
		t.Errorf("expected Fanuc-dialect output (%% / O-number / M30), got:\n%s", res.GCode)
	}
}

// TestEnginePocketRoutesAroundIsland checks a pocket over a plate with a hole passes the inner
// contour to the pocket op as an island, so the clearing routes around it.
func TestEnginePocketRoutesAroundIsland(t *testing.T) {
	h := &recordingHost{sectionWires: []wire.WirePolyline{
		{Points: []float64{0, 0, 0.5, 4, 0, 0.5, 4, 4, 0.5, 0, 4, 0.5, 0, 0, 0.5}, Closed: true},
		{Points: []float64{1.5, 1.5, 0.5, 2.5, 1.5, 0.5, 2.5, 2.5, 0.5, 1.5, 2.5, 0.5, 1.5, 1.5, 0.5}, Closed: true},
	}}
	job, _, err := NewEngine(h).SetPost("grbl").buildPocketJob(0)
	if err != nil {
		t.Fatalf("buildPocketJob: %v", err)
	}
	op, ok := job.Operations[0].(*PocketOp)
	if !ok {
		t.Fatalf("first op is not a pocket: %T", job.Operations[0])
	}
	if len(op.Islands) != 1 {
		t.Fatalf("pocket should carry the hole as 1 island, got %d", len(op.Islands))
	}
	if op.Islands[0].Area() >= op.Boundary.Area() {
		t.Errorf("the island should be smaller than the pocket boundary")
	}
}

// TestEngineProfileMachinesHoles checks profiling contours the outer outline plus each inner
// hole — one ProfileOp per contour, the holes cut inside.
func TestEngineProfileMachinesHoles(t *testing.T) {
	// a 4×4 cm plate (outer) with a 1×1 cm square hole near the corner (cm coords).
	h := &recordingHost{sectionWires: []wire.WirePolyline{
		{Points: []float64{0, 0, 0.5, 4, 0, 0.5, 4, 4, 0.5, 0, 4, 0.5, 0, 0, 0.5}, Closed: true},
		{Points: []float64{1, 1, 0.5, 2, 1, 0.5, 2, 2, 0.5, 1, 2, 0.5, 1, 1, 0.5}, Closed: true},
	}}
	e := NewEngine(h).SetPost("grbl")
	job, _, err := e.buildProfileJob(0)
	if err != nil {
		t.Fatalf("buildProfileJob: %v", err)
	}
	if len(job.Operations) != 2 {
		t.Fatalf("want 2 profile ops (outer + 1 hole), got %d", len(job.Operations))
	}
	outer, ok := job.Operations[0].(*ProfileOp)
	if !ok || outer.Side != "outside" {
		t.Errorf("first op should be the outer outline profiled outside, got %+v", job.Operations[0])
	}
	hole, ok := job.Operations[1].(*ProfileOp)
	if !ok || hole.Side != "inside" {
		t.Errorf("second op should be the hole profiled inside, got %+v", job.Operations[1])
	}
	// the hole's contour is smaller than the outer outline (sorted largest-first).
	if hole.Boundary.Area() >= outer.Boundary.Area() {
		t.Errorf("hole contour (%g) should be smaller than the outer outline (%g)", hole.Boundary.Area(), outer.Boundary.Area())
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
	if !h.called(wire.MethodCommandsCreate) {
		t.Errorf("Setup must register the CAM commands; got %v", h.methods)
	}
	// Setup must NOT open any window: the panels/browsers open on demand from the CAM tab.
	if h.called(wire.MethodDockableWindowsSet) {
		t.Error("Setup must not open a window by default")
	}
	// Every command lands on the CAM tab of the part ribbon, on a named panel.
	for _, c := range h.createdCmds {
		if c.Ribbon != types.PartRibbon || c.Tab != camRibbonTab || c.Category == "" {
			t.Errorf("command %q placed at ribbon=%q tab=%q panel=%q, want the part ribbon's CAM tab on a named panel", c.ID, c.Ribbon, c.Tab, c.Category)
		}
	}
	// A cutting tool carries the add-in's own inline glyph.
	for _, c := range h.createdCmds {
		if c.ID == GeneratePocketCommandID {
			if c.IconSVG == "" || c.ButtonStyle != types.LargeIconButton {
				t.Errorf("the Pocket tool should be a large icon button with an inline SVG glyph, got style=%v svgLen=%d", c.ButtonStyle, len(c.IconSVG))
			}
		}
	}
}

// TestRibbonLayoutCoversEveryCommand guards that every registered command has a ribbon spot (so none
// lands on an unnamed panel) and that every referenced glyph resolves to a bundled asset.
func TestRibbonLayoutCoversEveryCommand(t *testing.T) {
	for _, c := range camCommands {
		spot, ok := camRibbonSpots[c.id]
		if !ok {
			t.Errorf("command %q has no ribbon spot", c.id)
			continue
		}
		if spot.panel == "" {
			t.Errorf("command %q has an empty panel", c.id)
		}
		if spot.icon != "" && iconSVG(spot.icon) == "" {
			t.Errorf("command %q references glyph %q with no bundled icons/%s.svg", c.id, spot.icon, spot.icon)
		}
	}
}

// TestEveryCommandIsAnIconButton pins the requirement that EVERY CAM command carries an SVG glyph
// (no bare text buttons): a frequently used cutting tool is a large icon button, a less-used action
// (modify/dress-up/tool-library) a small one, but both have a resolvable icon. A text-only spot or a
// missing glyph regresses the all-icons ribbon.
func TestEveryCommandIsAnIconButton(t *testing.T) {
	for _, c := range camCommands {
		spot := camRibbonSpots[c.id]
		if spot.icon == "" {
			t.Errorf("command %q has no icon — every button must carry an SVG glyph", c.id)
			continue
		}
		if iconSVG(spot.icon) == "" {
			t.Errorf("command %q references glyph %q with no bundled icons/%s.svg", c.id, spot.icon, spot.icon)
		}
		if !spot.style.ShowsIcon() {
			t.Errorf("command %q uses style %v, want an icon button (small or large)", c.id, spot.style)
		}
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
	// A tool-list header precedes the operation object.
	if len(objs) != 2 {
		t.Fatalf("want 2 objects (tool-list header + op), got %d", len(objs))
	}
	if objs[0].Label != "Tool list" {
		t.Errorf("first object should be the tool-list header, got %q", objs[0].Label)
	}
	names := commandNames(objs[1].Path.Commands)
	// A per-op estimate comment leads the block, then the tool select, reverse spindle, op body.
	if len(names) == 0 || !strings.HasPrefix(names[0], "(") {
		t.Errorf("first command should be the estimate comment, got %v", names)
	}
	want := []string{"M6", "M4", "G80"}
	if strings.Join(names[1:], ",") != strings.Join(want, ",") {
		t.Errorf("commands after the comment = %v, want %v", names[1:], want)
	}
}

// TestPostArgsWorkOffset checks the engine threads the chosen work coordinate system into the
// post arguments, leaving the default (G54) implicit.
func TestPostArgsWorkOffset(t *testing.T) {
	e := NewEngine(&recordingHost{})
	if got := e.postArgs(); contains(got, "work-offset") {
		t.Errorf("default work offset should add no arg, got %q", got)
	}
	e.workOffset = 3 // G56
	if got := e.postArgs(); !contains(got, "--work-offset=G56") {
		t.Errorf("work offset 3 should pass --work-offset=G56, got %q", got)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
