// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"time"

	"oblikovati.org/api/client"
	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// CAM toolpath simulator (FreeCAD's Path Simulator): play the posted program back, animating a
// tool marker along the toolpath while the traced portion fills in. A panel offers play/pause,
// step, reset and a speed; a ticker goroutine advances the position and redraws. This is path
// playback, not material removal (a heavier voxel simulation is a later refinement).

const (
	SimPanelID  = "com.oblikovati.cam.sim.panel"
	SimTraceID  = "com.oblikovati.cam.sim.trace"  // toolpath already travelled (green)
	SimRemainID = "com.oblikovati.cam.sim.remain" // toolpath still to come (grey)
	SimToolID   = "com.oblikovati.cam.sim.tool"   // the tool marker (red)
)

// simTickInterval is the redraw period of the simulation while playing.
const simTickInterval = 40 * time.Millisecond

// simulateAction opens the simulator on the last posted program: extract its toolpath, draw the
// first frame paused, and show the controls.
func (e *Engine) simulateAction() (*JobResult, error) {
	e.mu.Lock()
	path := toolpathFromGCode(e.lastGCode)
	e.mu.Unlock()
	if len(path) < 2 {
		return nil, fmt.Errorf("no toolpath to simulate — generate and post a job first")
	}
	e.mu.Lock()
	e.simPath, e.simIdx, e.simRunning = path, 0, false
	if e.simSpeed == 0 {
		e.simSpeed = speedValue("Normal")
	}
	e.simGen++
	e.mu.Unlock()
	e.drawSimFrame()
	if _, err := e.showSimPanel(); err != nil {
		return nil, err
	}
	return &JobResult{Summary: fmt.Sprintf("CAM: simulating %d toolpath moves.", len(path))}, nil
}

// simPlayPauseAction toggles playback; starting from the end replays from the start.
func (e *Engine) simPlayPauseAction() (*JobResult, error) {
	e.mu.Lock()
	e.simRunning = !e.simRunning
	if e.simRunning && e.simIdx >= len(e.simPath)-1 {
		e.simIdx = 0
	}
	running, gen := e.simRunning, e.simGen
	e.mu.Unlock()
	if running {
		go e.simTick(gen)
	}
	e.drawSimFrame()
	_, _ = e.showSimPanel()
	return &JobResult{Summary: "CAM: simulator " + playWord(running) + "."}, nil
}

// simStepAction pauses and advances one move.
func (e *Engine) simStepAction() (*JobResult, error) {
	e.mu.Lock()
	e.simRunning = false
	if e.simIdx < len(e.simPath)-1 {
		e.simIdx++
	}
	e.mu.Unlock()
	e.drawSimFrame()
	_, _ = e.showSimPanel()
	return &JobResult{Summary: "CAM: stepped."}, nil
}

// simResetAction pauses and rewinds to the start.
func (e *Engine) simResetAction() (*JobResult, error) {
	e.mu.Lock()
	e.simRunning, e.simIdx = false, 0
	e.mu.Unlock()
	e.drawSimFrame()
	_, _ = e.showSimPanel()
	return &JobResult{Summary: "CAM: simulator reset."}, nil
}

// closeSimAction stops playback, clears the overlay, and hides the panel.
func (e *Engine) closeSimAction() (*JobResult, error) {
	e.mu.Lock()
	e.simRunning = false
	e.simGen++ // retire any running tick loop
	e.mu.Unlock()
	for _, id := range []string{SimTraceID, SimRemainID, SimToolID} {
		_ = e.api.Graphics().Delete(id)
	}
	if _, err := e.api.DockableWindows().SetVisible(SimPanelID, false); err != nil {
		return nil, err
	}
	return &JobResult{Summary: "CAM: simulator closed."}, nil
}

// simTick advances the playback on a timer while it stays the current generation and is running,
// redrawing each step and refreshing the panel once at the end.
func (e *Engine) simTick(gen int) {
	ticker := time.NewTicker(simTickInterval)
	defer ticker.Stop()
	for range ticker.C {
		e.mu.Lock()
		if e.simGen != gen || !e.simRunning {
			e.mu.Unlock()
			return
		}
		e.simIdx += e.simSpeed
		done := e.simIdx >= len(e.simPath)-1
		if done {
			e.simIdx, e.simRunning = len(e.simPath)-1, false
		}
		e.mu.Unlock()
		e.drawSimFrame()
		if done {
			_, _ = e.showSimPanel()
			return
		}
	}
}

// drawSimFrame redraws the traced and remaining toolpath and the tool marker at the current move.
func (e *Engine) drawSimFrame() {
	e.mu.Lock()
	idx, path := e.simIdx, e.simPath
	e.mu.Unlock()
	if len(path) < 2 {
		return
	}
	tc, ti := polylineLines(path[:idx+1])
	rc, ri := polylineLines(path[idx:])
	_, _ = e.api.Graphics().AddLines(SimTraceID, tc, ti, []float32{0.1, 0.9, 0.2, 1})
	_, _ = e.api.Graphics().AddLines(SimRemainID, rc, ri, []float32{0.55, 0.55, 0.6, 0.6})
	tool := []float64{path[idx].X / cmToMM, path[idx].Y / cmToMM, path[idx].Z / cmToMM}
	_, _ = e.api.Graphics().AddPoints(SimToolID, tool, types.GraphicsPointSquare, []float32{1, 0.2, 0.1, 1})
}

// showSimPanel renders (or refreshes) the simulator controls.
func (e *Engine) showSimPanel() (wire.OKResult, error) {
	e.mu.Lock()
	idx, total, running, speed := e.simIdx, len(e.simPath), e.simRunning, e.simSpeed
	e.mu.Unlock()
	thirds := []types.GridTrack{client.TrackFr(1), client.TrackFr(1), client.TrackFr(1)}
	return e.api.DockableWindows().Set(wire.DockableWindowSpec{
		ID: SimPanelID, Title: "CAM Simulator", Dock: types.DockRight, Visible: true,
		Controls: []wire.PanelControlSpec{
			client.PanelLabel("sim_progress", fmt.Sprintf("Move %d / %d", idx+1, total)),
			client.PanelGrid("sim_btns", thirds, 4, 4,
				client.PanelButton("sim_play", playButton(running), SimPlayPauseCommandID),
				client.PanelButton("sim_step", "Step", SimStepCommandID),
				client.PanelButton("sim_reset", "Reset", SimResetCommandID)),
			client.PanelDropdown("sim_speed", "Speed", speedOptions(), speedLabel(speed)),
			client.PanelButton("sim_close", "Close", SimCloseCommandID),
		},
	})
}

// applySimEdit applies one simulator-panel edit (the speed).
func (e *Engine) applySimEdit(controlID, value string) {
	if controlID == "sim_speed" {
		e.mu.Lock()
		e.simSpeed = speedValue(value)
		e.mu.Unlock()
	}
}

// playWord / playButton render the running state.
func playWord(running bool) string {
	if running {
		return "playing"
	}
	return "paused"
}

func playButton(running bool) string {
	if running {
		return "Pause"
	}
	return "Play"
}

// speedOptions and the value/label mapping translate the speed dropdown to moves-per-tick.
func speedOptions() []string { return []string{"Slow", "Normal", "Fast"} }

func speedValue(label string) int {
	switch label {
	case "Slow":
		return 1
	case "Fast":
		return 12
	default:
		return 4
	}
}

func speedLabel(value int) string {
	switch {
	case value <= 1:
		return "Slow"
	case value >= 12:
		return "Fast"
	default:
		return "Normal"
	}
}
