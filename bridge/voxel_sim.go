// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"math"

	"oblikovati.org/api/types"
	"oblikovati.org/cam/bridge/gcode"
)

// Material-removal driver for the simulator: the toolpath playback (simulator.go) animates a marker,
// while this layer carves a voxel stock so the user sees material disappear. It re-runs the job's
// (host-free) path generation to recover the exact cutter and toolpath per operation, voxelises the
// stock, and sweeps cuts incrementally as playback advances.

// voxelCellCap bounds the grid so a large stock can't exhaust memory; chooseVoxelRes coarsens past
// it. ~3M cells is a few hundred KB of bitset and meshes in well under a frame.
const voxelCellCap = 3_000_000

// voxelGrowth is the per-step coarsening factor (≈ cube root of 2) used to bring an over-cap grid
// back under the cell cap.
const voxelGrowth = 1.2599

// chooseVoxelRes picks a cell size (mm): half the smallest cutter radius so a tool spans a few
// cells, coarsened until the grid fits voxelCellCap. With no tool it falls back to the stock's
// longest axis over 64.
func chooseVoxelRes(min, max gcode.Vector3, minToolRadius float64) float64 {
	ex, ey, ez := max.X-min.X, max.Y-min.Y, max.Z-min.Z
	res := minToolRadius / 2
	if res <= 0 {
		res = math.Max(ex, math.Max(ey, ez)) / 64
	}
	if res <= 0 {
		res = 1
	}
	for axisCells(ex, res)*axisCells(ey, res)*axisCells(ez, res) > voxelCellCap {
		res *= voxelGrowth
	}
	return res
}

// cutPoints is the playback polyline for a move list: the first move's start, then every move's end.
func cutPoints(cuts []voxelMove) []gcode.Vector3 {
	if len(cuts) == 0 {
		return nil
	}
	pts := make([]gcode.Vector3, 0, len(cuts)+1)
	pts = append(pts, cuts[0].from)
	for _, c := range cuts {
		pts = append(pts, c.to)
	}
	return pts
}

// minCutterRadius is the smallest positive cutter radius across the moves (the feature size the grid
// must resolve), or 0 when none.
func minCutterRadius(cuts []voxelMove) float64 {
	min := 0.0
	for _, c := range cuts {
		if c.cutter.Radius > 0 && (min == 0 || c.cutter.Radius < min) {
			min = c.cutter.Radius
		}
	}
	return min
}

// buildMaterialSim prepares the voxel view from the last generated job: regenerate its paths,
// flatten them to cuts, size and create the stock grid. Reports whether a material sim is available
// (it needs a job that produces cutting moves).
func (e *Engine) buildMaterialSim() bool {
	if e.lastJob == nil {
		return false
	}
	results, err := e.lastJob.GenerateAll()
	if err != nil {
		return false
	}
	stock := e.lastJob.Stock
	cuts := flattenCuts(results, stock.Max.Z-stock.Min.Z)
	if len(cuts) == 0 {
		return false
	}
	e.simCuts, e.simStock = cuts, stock
	e.voxelRes = chooseVoxelRes(stock.Min, stock.Max, minCutterRadius(cuts))
	e.rebuildVoxelGrid()
	e.simPath, e.simFeed, e.simIdx = cutPoints(cuts), nil, 0 // material view draws the mesh, not coloured lines
	return true
}

// rebuildVoxelGrid recreates the solid stock grid and rewinds the cut cursor (used on open and when
// scrubbing playback backwards).
func (e *Engine) rebuildVoxelGrid() {
	e.voxel = NewVoxelGrid(e.simStock.Min, e.simStock.Max, e.voxelRes)
	e.voxelCursor = 0
}

// advanceVoxelTo carves the grid so exactly the first idx cuts are applied, rebuilding first when
// playback has moved backwards.
func (e *Engine) advanceVoxelTo(idx int) {
	if idx < e.voxelCursor {
		e.rebuildVoxelGrid()
	}
	for e.voxelCursor < idx && e.voxelCursor < len(e.simCuts) {
		c := e.simCuts[e.voxelCursor]
		sweepSegment(e.voxel, c.cutter, c.from, c.to)
		e.voxelCursor++
	}
}

// drawVoxelFrame carves to the current move and pushes the remaining-stock mesh plus the tool marker
// into the viewport (coordinates converted mm→host cm).
func (e *Engine) drawVoxelFrame() {
	e.mu.Lock()
	idx := e.simIdx
	if e.voxel == nil || idx < 0 || idx >= len(e.simPath) {
		e.mu.Unlock()
		return
	}
	e.advanceVoxelTo(idx)
	coords, indices := voxelSurfaceMesh(e.voxel)
	tip := e.simPath[idx]
	e.mu.Unlock()
	scaleToHost(coords)
	_, _ = e.api.Graphics().AddMesh(SimStockID, coords, indices, []float32{0.72, 0.66, 0.5, 1})
	marker := []float64{tip.X / cmToMM, tip.Y / cmToMM, tip.Z / cmToMM}
	_, _ = e.api.Graphics().AddPoints(SimToolID, marker, types.GraphicsPointSquare, []float32{1, 0.2, 0.1, 1})
}

// scaleToHost converts a millimetre coordinate stream in place to the host's centimetres.
func scaleToHost(coords []float64) {
	for i := range coords {
		coords[i] /= cmToMM
	}
}

// materialStatus is the simulator progress line in material mode: the move counter plus how much
// stock remains.
func (e *Engine) materialStatus() string {
	if e.voxel == nil {
		return ""
	}
	remain := 100.0 * float64(e.voxel.Count()) / float64(e.voxel.Total())
	return fmt.Sprintf("  ·  %.0f%% stock", remain)
}
