// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"math"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
	"oblikovati.org/cam/bridge/ocl"
)

// Waterline (z-level) finishing defaults (millimetres).
const (
	waterStepOver = 1.0 // grid sampling and contour spacing
	waterStepDown = 1.0 // Z spacing between levels
)

// RunWaterlineJobOnHost finishes the body with constant-Z (waterline) passes: it meshes the
// part, samples the drop-cutter surface on a grid (OpenCAMLib), slices that surface at each Z
// level into contour loops, and posts the level toolpath with an overlay.
func (e *Engine) RunWaterlineJobOnHost(bodyIndex int) (*JobResult, error) {
	tris, stock, err := e.meshAndStock(bodyIndex)
	if err != nil {
		return nil, err
	}
	job := e.newMillJob(bodyIndex, stock)
	job.Tools[0].Tool.ShapeType = "ballend"
	diameter := job.Tools[0].Tool.Diameter

	cut := e.cutting()
	hf, err := e.dropCutterField(tris, stock, diameter)
	if err != nil {
		return nil, err
	}
	stepDown := levelSpacing(cut, waterStepDown)
	levels := extractLevels(hf, stepDown)
	job.Operations = []Operation{&WaterlineOp{
		OpBase:   e.millEnvelope("Waterline", stock),
		StepOver: waterStepOver, StepDown: stepDown, Levels: levels,
	}}
	return e.postPreviewResult(job, fmt.Sprintf("waterline-finished the surface (%d levels)", len(levels)))
}

// dropCutterField samples the cutter-location surface on a regular grid by dropping the cutter
// along scan lines stepped (and sampled) at waterStepOver, then interpolating each scan line
// onto the grid's Y nodes.
func (e *Engine) dropCutterField(tris []ocl.Triangle, stock Stock, diameter float64) (*gen.Heightfield, error) {
	lines := scanLines(stock, waterStepOver)
	rows, err := e.surfacer.DropCutter(tris, diameter, ballCutterLength, stock.BottomZ(), waterStepOver, lines)
	if err != nil {
		return nil, fmt.Errorf("drop-cutter over %d triangles / %d passes: %w", len(tris), len(lines), err)
	}
	ny := int(math.Round((stock.Max.Y-stock.Min.Y)/waterStepOver)) + 1
	return heightfieldFromRows(rows, stock.Min.X, stock.Min.Y, waterStepOver, ny), nil
}

// heightfieldFromRows resamples drop-cutter scan-line rows onto a regular XY grid.
func heightfieldFromRows(rows [][]ocl.Point3, x0, y0, step float64, ny int) *gen.Heightfield {
	nx := len(rows)
	hf := &gen.Heightfield{X0: x0, Y0: y0, Step: step, NX: nx, NY: ny, Z: make([]float64, nx*ny)}
	for i, row := range rows {
		for j := 0; j < ny; j++ {
			hf.Z[i*ny+j] = interpRowZ(row, y0+float64(j)*step)
		}
	}
	return hf
}

// interpRowZ linearly interpolates a scan line's tip Z at the given Y (the row is ascending in
// Y), returning NaN outside the row's sampled span.
func interpRowZ(row []ocl.Point3, y float64) float64 {
	if len(row) == 0 || y < row[0].Y-1e-6 || y > row[len(row)-1].Y+1e-6 {
		return math.NaN()
	}
	for k := 1; k < len(row); k++ {
		if row[k].Y >= y {
			a, b := row[k-1], row[k]
			if d := b.Y - a.Y; d > 0 {
				t := (y - a.Y) / d
				return a.Z + t*(b.Z-a.Z)
			}
			return a.Z
		}
	}
	return row[len(row)-1].Z
}

// extractLevels slices the height field from its top down to its floor at stepDown spacing,
// returning the non-empty level contour sets (each loop lifted to its Z).
func extractLevels(hf *gen.Heightfield, stepDown float64) []gen.LevelLoops {
	zBot, zTop, ok := fieldRange(hf) // fieldRange returns (min, max)
	if !ok {
		return nil
	}
	var levels []gen.LevelLoops
	for _, z := range gen.DepthLevels(zTop, zBot, stepDown) {
		var loops [][]gcode.Vector3
		for _, lp := range hf.Contour(z) {
			if len(lp) < 3 {
				continue
			}
			v := make([]gcode.Vector3, len(lp))
			for k, p := range lp {
				v[k] = gcode.Vector3{X: p[0], Y: p[1], Z: z}
			}
			loops = append(loops, v)
		}
		if len(loops) > 0 {
			levels = append(levels, gen.LevelLoops{Z: z, Loops: loops})
		}
	}
	return levels
}

// fieldRange returns the min and max defined (non-NaN) height in the field.
func fieldRange(hf *gen.Heightfield) (lo, hi float64, ok bool) {
	lo, hi = math.Inf(1), math.Inf(-1)
	for _, z := range hf.Z {
		if math.IsNaN(z) {
			continue
		}
		lo, hi, ok = math.Min(lo, z), math.Max(hi, z), true
	}
	return lo, hi, ok
}
