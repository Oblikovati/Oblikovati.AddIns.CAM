// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"time"

	"oblikovati.org/api/client"
	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
	"oblikovati.org/cam/bridge/gcode"
)

// CAM toolpath simulator (FreeCAD's Path Simulator): play the posted program back, animating a
// tool marker along the toolpath while the traced portion fills in. A panel offers play/pause,
// step, reset and a speed; a ticker goroutine advances the position and redraws. This is path
// playback, not material removal (a heavier voxel simulation is a later refinement).

// The playback path is drawn as four overlays — cutting moves vs rapids, each split into the
// travelled and remaining portion — so move type (green cut / blue rapid) and progress (bright
// travelled / dim remaining) read at once. See pathOverlays.
const (
	SimPanelID     = "com.oblikovati.cam.sim.panel"
	SimFeedDoneID  = "com.oblikovati.cam.sim.feed.done"  // travelled cutting moves (bright green)
	SimRapidDoneID = "com.oblikovati.cam.sim.rapid.done" // travelled rapids (bright blue)
	SimFeedTodoID  = "com.oblikovati.cam.sim.feed.todo"  // remaining cutting moves (dim green)
	SimRapidTodoID = "com.oblikovati.cam.sim.rapid.todo" // remaining rapids (dim blue)
	SimToolID      = "com.oblikovati.cam.sim.tool"       // the tool marker (red)
	SimStockID     = "com.oblikovati.cam.sim.stock"      // the remaining material mesh (material mode)
)

// pathOverlay is one playback line overlay: which segments it holds (cutting vs rapid, travelled vs
// remaining) and the colour to draw them.
type pathOverlay struct {
	id    string
	feed  bool
	done  bool
	color []float32
}

// pathOverlays groups the playback segments by move type and progress. Cutting moves are green,
// rapids blue; travelled segments are opaque, remaining ones dim.
var pathOverlays = []pathOverlay{
	{SimFeedDoneID, true, true, []float32{0.1, 0.9, 0.2, 1}},
	{SimRapidDoneID, false, true, []float32{0.3, 0.55, 0.95, 1}},
	{SimFeedTodoID, true, false, []float32{0.1, 0.9, 0.2, 0.3}},
	{SimRapidTodoID, false, false, []float32{0.3, 0.55, 0.95, 0.3}},
}

// pathOverlayIDs is the graphics ids of every playback line overlay (for clearing).
func pathOverlayIDs() []string {
	ids := make([]string, len(pathOverlays))
	for i, o := range pathOverlays {
		ids[i] = o.id
	}
	return ids
}

// simTickInterval is the redraw period of the simulation while playing.
const simTickInterval = 40 * time.Millisecond

// simulateAction opens the simulator on the last posted program: prefer the material-removal view
// (carving a voxel stock) when the job can produce one, else plain path playback. Draws the first
// frame paused and shows the controls.
func (e *Engine) simulateAction() (*JobResult, error) {
	if !e.prepareSim() {
		return nil, fmt.Errorf("no toolpath to simulate — generate and post a job first")
	}
	e.mu.Lock()
	e.simRunning = false
	if e.simSpeed == 0 {
		e.simSpeed = speedValue("Normal")
	}
	e.simGen++
	n, material, res := len(e.simPath), e.simMaterial, e.voxelRes
	e.mu.Unlock()
	e.drawSimFrame()
	if _, err := e.showSimPanel(); err != nil {
		return nil, err
	}
	return &JobResult{Summary: simSummary(n, material, res)}, nil
}

// simSummary is the status line for an opened simulator; the material view names the voxel cell size
// so any coarsening of a large stock is visible.
func simSummary(moves int, material bool, res float64) string {
	if material {
		return fmt.Sprintf("CAM: simulating %d moves (Material, %.2f mm voxels).", moves, res)
	}
	return fmt.Sprintf("CAM: simulating %d moves (Path).", moves)
}

// prepareSim builds the playback sequence, preferring material removal when the last job yields
// cuts, else path playback from the posted G-code. Reports whether anything is playable.
func (e *Engine) prepareSim() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.buildMaterialSim() {
		e.simMaterial = true
		return true
	}
	e.simMaterial, e.voxel = false, nil
	e.simPath, e.simFeed = motionWithKinds(e.lastGCode)
	e.simIdx = 0
	return len(e.simPath) >= 2
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
	e.voxel = nil
	e.mu.Unlock()
	for _, id := range append(pathOverlayIDs(), SimStockID, SimToolID) {
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

// drawSimFrame draws the current frame in the active view — the carved stock in material mode, the
// traced/remaining polyline otherwise.
func (e *Engine) drawSimFrame() {
	e.mu.Lock()
	material := e.simMaterial
	e.mu.Unlock()
	if material {
		e.drawVoxelFrame()
		return
	}
	e.drawPathFrame()
}

// drawPathFrame redraws the toolpath coloured by move type (cut vs rapid) and progress, plus the
// tool marker at the current move.
func (e *Engine) drawPathFrame() {
	e.mu.Lock()
	idx, path, feed := e.simIdx, e.simPath, e.simFeed
	e.mu.Unlock()
	if len(path) < 2 {
		return
	}
	for _, o := range pathOverlays {
		e.drawPathOverlay(o, path, feed, idx)
	}
	e.drawToolMarker(path[idx])
}

// drawPathOverlay draws (or clears, when empty) the segments belonging to one overlay group.
func (e *Engine) drawPathOverlay(o pathOverlay, path []gcode.Vector3, feed []bool, idx int) {
	coords, indices := segmentLines(path, func(i int) bool {
		return segmentIsFeed(feed, i) == o.feed && (i < idx) == o.done
	})
	if len(indices) == 0 {
		_ = e.api.Graphics().Delete(o.id)
		return
	}
	_, _ = e.api.Graphics().AddLines(o.id, coords, indices, o.color)
}

// segmentIsFeed reports whether the segment from point i to i+1 is a cutting move (the move into
// point i+1).
func segmentIsFeed(feed []bool, i int) bool {
	return i+1 < len(feed) && feed[i+1]
}

// drawToolMarker draws the red tool marker at a point (mm → host cm).
func (e *Engine) drawToolMarker(p gcode.Vector3) {
	tool := []float64{p.X / cmToMM, p.Y / cmToMM, p.Z / cmToMM}
	_, _ = e.api.Graphics().AddPoints(SimToolID, tool, types.GraphicsPointSquare, []float32{1, 0.2, 0.1, 1})
}

// showSimPanel renders (or refreshes) the simulator controls.
func (e *Engine) showSimPanel() (wire.OKResult, error) {
	e.mu.Lock()
	idx, total, running, speed := e.simIdx, len(e.simPath), e.simRunning, e.simSpeed
	material, status := e.simMaterial, ""
	if material {
		status = e.materialStatus()
	}
	e.mu.Unlock()
	thirds := []types.GridTrack{client.TrackFr(1), client.TrackFr(1), client.TrackFr(1)}
	return e.api.DockableWindows().Set(wire.DockableWindowSpec{
		ID: SimPanelID, Title: "CAM Simulator", Dock: types.DockRight, Visible: true,
		Controls: []wire.PanelControlSpec{
			client.PanelLabel("sim_progress", fmt.Sprintf("Move %d / %d%s", idx+1, total, status)),
			client.PanelDropdown("sim_view", "View", simViewOptions(), simViewLabel(material)),
			client.PanelGrid("sim_btns", thirds, 4, 4,
				client.PanelButton("sim_play", playButton(running), SimPlayPauseCommandID),
				client.PanelButton("sim_step", "Step", SimStepCommandID),
				client.PanelButton("sim_reset", "Reset", SimResetCommandID)),
			client.PanelDropdown("sim_speed", "Speed", speedOptions(), speedLabel(speed)),
			client.PanelButton("sim_close", "Close", SimCloseCommandID),
		},
	})
}

// applySimEdit applies one simulator-panel edit: the playback speed or the view mode.
func (e *Engine) applySimEdit(controlID, value string) {
	switch controlID {
	case "sim_speed":
		e.mu.Lock()
		e.simSpeed = speedValue(value)
		e.mu.Unlock()
	case "sim_view":
		e.switchSimView(value == "Material")
	}
}

// switchSimView rebuilds the playback for the chosen view, clears the other view's overlay and
// redraws — used by the View dropdown.
func (e *Engine) switchSimView(material bool) {
	e.mu.Lock()
	if material && e.buildMaterialSim() {
		e.simMaterial = true
	} else {
		e.simMaterial, e.voxel = false, nil
		e.simPath, e.simFeed = motionWithKinds(e.lastGCode)
		e.simIdx = 0
	}
	e.simGen++
	e.mu.Unlock()
	e.clearSimOverlays()
	e.drawSimFrame()
	_, _ = e.showSimPanel()
}

// clearSimOverlays removes every simulator overlay except the tool marker (which the next frame
// overwrites), so switching views never leaves the previous view's geometry behind.
func (e *Engine) clearSimOverlays() {
	for _, id := range append(pathOverlayIDs(), SimStockID) {
		_ = e.api.Graphics().Delete(id)
	}
}

// simViewOptions and simViewLabel describe the View dropdown (carved stock vs. path playback).
func simViewOptions() []string { return []string{"Material", "Path"} }

func simViewLabel(material bool) string {
	if material {
		return "Material"
	}
	return "Path"
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
