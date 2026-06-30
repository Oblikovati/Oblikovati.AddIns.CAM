// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// simProgram is a tiny posted program with five motion moves, used to drive the simulator.
const simProgram = "G0 X0 Y0 Z5\nM3 S1000\nG1 Z0 F100\nG1 X10\nG1 Y10\nG0 Z5"

// TestSimulateActionErrorsWithoutToolpath checks the simulator refuses to open when no program
// has been posted (its message names the missing precondition).
func TestSimulateActionErrorsWithoutToolpath(t *testing.T) {
	e := NewEngine(&recordingHost{})
	if _, err := e.simulateAction(); err == nil {
		t.Fatal("expected an error with no posted toolpath")
	}
}

// TestSimulateActionDrawsPathAndPanel checks opening the simulator draws the trace, remaining and
// tool-marker overlays and shows the control panel.
func TestSimulateActionDrawsPathAndPanel(t *testing.T) {
	h := &recordingHost{}
	e := NewEngine(h)
	e.lastGCode = simProgram

	if _, err := e.simulateAction(); err != nil {
		t.Fatalf("simulateAction: %v", err)
	}
	// simProgram has cutting moves and a trailing rapid, so at the start both remaining-move overlays
	// and the tool marker are drawn.
	for _, id := range []string{SimFeedTodoID, SimRapidTodoID, SimToolID} {
		if !hasGraphic(h, id) {
			t.Errorf("overlay %q not drawn", id)
		}
	}
	win, ok := lastDock(h, SimPanelID)
	if !ok {
		t.Fatal("simulator panel not shown")
	}
	if win.Title != "CAM Simulator" || win.Dock != types.DockRight {
		t.Errorf("panel id/title/dock = %q/%q/%v", win.ID, win.Title, win.Dock)
	}
}

// TestPlaybackColorsByMoveType checks the travelled toolpath is split into a green cutting overlay
// and a blue rapid overlay once playback reaches the end.
func TestPlaybackColorsByMoveType(t *testing.T) {
	h := &recordingHost{}
	e := NewEngine(h)
	e.lastGCode = simProgram
	if _, err := e.simulateAction(); err != nil {
		t.Fatalf("simulateAction: %v", err)
	}
	for i := 0; i < len(e.simPath); i++ {
		_, _ = e.simStepAction() // play to the end so every segment is travelled
	}
	if got := graphicColor(h, SimFeedDoneID); !colorEq(got, []float32{0.1, 0.9, 0.2, 1}) {
		t.Errorf("travelled cut colour = %v, want green", got)
	}
	if got := graphicColor(h, SimRapidDoneID); !colorEq(got, []float32{0.3, 0.55, 0.95, 1}) {
		t.Errorf("travelled rapid colour = %v, want blue", got)
	}
}

// graphicColor returns the overall colour of the most recent client-graphics set for an id.
func graphicColor(h *recordingHost, id string) []float32 {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := len(h.graphicsArgs) - 1; i >= 0; i-- {
		a := h.graphicsArgs[i]
		if a.ClientId == id && len(a.Nodes) > 0 && len(a.Nodes[0].Primitives) > 0 {
			return a.Nodes[0].Primitives[0].Color
		}
	}
	return nil
}

func colorEq(got, want []float32) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestSimStepAdvancesAndReset checks stepping advances one move and reset rewinds to the start.
func TestSimStepAdvancesAndReset(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.lastGCode = simProgram
	if _, err := e.simulateAction(); err != nil {
		t.Fatalf("simulateAction: %v", err)
	}
	if e.simIdx != 0 {
		t.Fatalf("start index = %d, want 0", e.simIdx)
	}
	_, _ = e.simStepAction()
	_, _ = e.simStepAction()
	if e.simIdx != 2 {
		t.Errorf("after two steps index = %d, want 2", e.simIdx)
	}
	_, _ = e.simResetAction()
	if e.simIdx != 0 || e.simRunning {
		t.Errorf("after reset index/running = %d/%v, want 0/false", e.simIdx, e.simRunning)
	}
}

// TestSimStepStopsAtEnd checks stepping never runs off the end of the path.
func TestSimStepStopsAtEnd(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.lastGCode = simProgram
	if _, err := e.simulateAction(); err != nil {
		t.Fatalf("simulateAction: %v", err)
	}
	for i := 0; i < 20; i++ {
		_, _ = e.simStepAction()
	}
	if e.simIdx != len(e.simPath)-1 {
		t.Errorf("clamped index = %d, want %d", e.simIdx, len(e.simPath)-1)
	}
}

// TestCloseSimClearsOverlay checks closing the simulator deletes its overlays and hides the panel.
func TestCloseSimClearsOverlay(t *testing.T) {
	h := &recordingHost{}
	e := NewEngine(h)
	e.lastGCode = simProgram
	if _, err := e.simulateAction(); err != nil {
		t.Fatalf("simulateAction: %v", err)
	}
	if _, err := e.closeSimAction(); err != nil {
		t.Fatalf("closeSimAction: %v", err)
	}
	if !h.called(wire.MethodClientGraphicsDelete) {
		t.Error("overlays not deleted on close")
	}
}

// TestSpeedRoundTrip checks the speed label/value mapping is a stable round-trip.
func TestSpeedRoundTrip(t *testing.T) {
	for _, label := range speedOptions() {
		if got := speedLabel(speedValue(label)); got != label {
			t.Errorf("speedLabel(speedValue(%q)) = %q", label, got)
		}
	}
}

// hasGraphic reports whether a client-graphics set was recorded for the given id.
func hasGraphic(h *recordingHost, id string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, a := range h.graphicsArgs {
		if a.ClientId == id {
			return true
		}
	}
	return false
}

// lastDock returns the most recent dockable-window spec set for the given id.
func lastDock(h *recordingHost, id string) (wire.DockableWindowSpec, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := len(h.dockWindows) - 1; i >= 0; i-- {
		if h.dockWindows[i].ID == id {
			return h.dockWindows[i], true
		}
	}
	return wire.DockableWindowSpec{}, false
}
