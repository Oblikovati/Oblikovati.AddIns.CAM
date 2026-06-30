// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
	"time"

	"oblikovati.org/cam/bridge/gcode"
)

// materialJob is a one-profile job over a 20×20×10 stock — small enough to voxelise quickly and to
// carve visibly when the simulator runs.
func materialJob() *Job {
	return &Job{
		Stock: Stock{Min: gcode.Vector3{}, Max: gcode.Vector3{X: 20, Y: 20, Z: 10}},
		Tools: []ToolController{{HorizFeed: 300, VertFeed: 100, Tool: ToolBit{ShapeType: "endmill", Diameter: 6}}},
		Operations: []Operation{&ProfileOp{
			OpBase:   OpBase{OpLabel: "P", IsActive: true, ClearanceHeight: 15, SafeHeight: 12, StartDepth: 10, FinalDepth: 2},
			Side:     "outside",
			Climb:    true,
			Boundary: squarePoly(16),
		}},
	}
}

// TestSimulateDefaultsToMaterial checks the simulator opens in the material view when a job is
// available, drawing the carved-stock mesh.
func TestSimulateDefaultsToMaterial(t *testing.T) {
	h := &recordingHost{}
	e := NewEngine(h)
	e.lastJob = materialJob()
	e.lastGCode = simProgram // path fallback also available, but material must win

	res, err := e.simulateAction()
	if err != nil {
		t.Fatalf("simulateAction: %v", err)
	}
	if !e.simMaterial {
		t.Fatal("simulator did not default to material view with a job present")
	}
	if !hasGraphic(h, SimStockID) {
		t.Error("stock mesh not drawn")
	}
	if res.Summary == "" {
		t.Error("empty summary")
	}
}

// TestMaterialSimCarvesStock checks stepping the simulation removes material (the occupied cell
// count drops below the solid total).
func TestMaterialSimCarvesStock(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.lastJob = materialJob()
	if _, err := e.simulateAction(); err != nil {
		t.Fatalf("simulateAction: %v", err)
	}
	total := e.voxel.Total()
	for i := 0; i < len(e.simPath); i++ {
		_, _ = e.simStepAction()
	}
	if e.voxel.Count() >= total {
		t.Errorf("no material removed: %d of %d cells remain", e.voxel.Count(), total)
	}
}

// TestSwitchToPathViewDropsGrid checks toggling the View to Path clears the voxel grid and falls
// back to polyline playback.
func TestSwitchToPathViewDropsGrid(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.lastJob = materialJob()
	e.lastGCode = simProgram
	if _, err := e.simulateAction(); err != nil {
		t.Fatalf("simulateAction: %v", err)
	}
	e.applySimEdit("sim_view", "Path")
	if e.simMaterial || e.voxel != nil {
		t.Error("Path view did not drop the voxel grid")
	}
	if len(e.simPath) < 2 {
		t.Error("path playback sequence not rebuilt")
	}
	e.applySimEdit("sim_view", "Material") // toggle back
	if !e.simMaterial || e.voxel == nil {
		t.Error("Material view did not rebuild the voxel grid")
	}
}

// TestMaterialSimPlaybackCarves checks running the simulation (the animation tick loop) carves the
// stock to completion in material mode.
func TestMaterialSimPlaybackCarves(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.lastJob = materialJob()
	if _, err := e.simulateAction(); err != nil {
		t.Fatalf("simulateAction: %v", err)
	}
	e.applySimEdit("sim_speed", "Fast")
	total := e.voxel.Total()
	if _, err := e.simPlayPauseAction(); err != nil { // start playing
		t.Fatalf("simPlayPauseAction: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		e.mu.Lock()
		done := !e.simRunning
		e.mu.Unlock()
		if done {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if e.simIdx != len(e.simPath)-1 {
		t.Errorf("playback stopped at %d, want end %d", e.simIdx, len(e.simPath)-1)
	}
	if e.voxel.Count() >= total {
		t.Error("playback removed no material")
	}
}
