// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"math"

	"oblikovati.org/api/wire"
	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
	"oblikovati.org/cam/bridge/geom2d"
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
	cut := e.cutting()
	ball := indexForShape(job.Tools, "ballend")
	diameter := job.Tools[ball].Tool.Diameter
	stepDown := levelSpacing(cut, waterStepDown)
	levels, err := e.waterlineLevels(bodyIndex, diameter, stock, tris, stepDown)
	if err != nil {
		return nil, err
	}
	env := e.millEnvelope("Waterline", stock)
	env.ToolController = ball
	job.Operations = []Operation{&WaterlineOp{
		OpBase:   env,
		StepOver: waterStepOver, StepDown: stepDown, Levels: levels,
	}}
	return e.postPreviewResult(job, fmt.Sprintf("waterline-finished the surface (%d levels)", len(levels)))
}

// waterlineLevels builds the constant-Z tool-centre contours. It prefers the EXACT offset surface —
// the part faces offset outward by the ball radius (brep.offsetFaces), sectioned at each Z level
// (brep.sectionWithPlane) — so the loops come from analytic geometry. If that yields nothing (e.g. a
// surface whose offset self-intersects), it falls back to slicing the drop-cutter heightfield.
func (e *Engine) waterlineLevels(bodyIndex int, diameter float64, stock Stock, tris []ocl.Triangle, stepDown float64) ([]gen.LevelLoops, error) {
	if levels, err := e.offsetSurfaceLevels(bodyIndex, diameter/2, stock, stepDown); err == nil && len(levels) > 0 {
		return levels, nil
	}
	hf, err := e.dropCutterField(tris, stock, diameter)
	if err != nil {
		return nil, err
	}
	return extractLevels(hf, stepDown), nil
}

// offsetSurfaceLevels offsets the body's faces outward by the ball radius (the tool-centre surface)
// and sections that transient surface at each Z level into contour loops. Z and the ball radius are
// millimetres; the brep ops work in the host's centimetre unit, hence the cmToMM scaling.
func (e *Engine) offsetSurfaceLevels(bodyIndex int, ballRadius float64, stock Stock, stepDown float64) ([]gen.LevelLoops, error) {
	refs, err := e.api.Model().ReferenceKeys()
	if err != nil {
		return nil, fmt.Errorf("read reference keys: %w", err)
	}
	keys := bodyFaceKeys(refs, bodyIndex)
	if len(keys) == 0 {
		return nil, fmt.Errorf("body %d has no faces to offset", bodyIndex)
	}
	off, err := e.api.TransientBRep().OffsetFaces(wire.BrepOffsetFacesArgs{
		Source:   wire.BrepBodyRef{BodyIndex: &bodyIndex},
		FaceKeys: keys,
		Distance: ballRadius / cmToMM,
	})
	if err != nil {
		return nil, fmt.Errorf("offset body %d surface by the ball radius: %w", bodyIndex, err)
	}
	var levels []gen.LevelLoops
	// The cutter centre sits a ball radius above the highest point, so the top level is there.
	for _, z := range gen.DepthLevels(stock.Max.Z+ballRadius, stock.BottomZ(), stepDown) {
		section, err := e.api.TransientBRep().CreateIntersectionWithPlane(
			wire.BrepBodyRef{Handle: off.Handle}, []float64{0, 0, z / cmToMM}, []float64{0, 0, 1})
		if err != nil {
			return nil, fmt.Errorf("section the offset surface at z=%.2f: %w", z, err)
		}
		if loops := levelLoopsFromContours(sortedContours(section.Wires), z); len(loops) > 0 {
			levels = append(levels, gen.LevelLoops{Z: z, Loops: loops})
		}
	}
	return levels, nil
}

// bodyFaceKeys returns the reference keys of every face of the given document body.
func bodyFaceKeys(refs wire.ReferenceKeysResult, bodyIndex int) []string {
	if bodyIndex < 0 || bodyIndex >= len(refs.Bodies) {
		return nil
	}
	faces := refs.Bodies[bodyIndex].Faces
	keys := make([]string, 0, len(faces))
	for _, f := range faces {
		keys = append(keys, f.Key)
	}
	return keys
}

// levelLoopsFromContours turns the XY section polygons (mm) into closed tool-centre loops at height z.
func levelLoopsFromContours(polys []geom2d.Polygon, z float64) [][]gcode.Vector3 {
	var loops [][]gcode.Vector3
	for _, poly := range polys {
		if len(poly) < 3 {
			continue
		}
		loop := make([]gcode.Vector3, len(poly)+1)
		for i, p := range poly {
			loop[i] = gcode.Vector3{X: p.X, Y: p.Y, Z: z}
		}
		loop[len(poly)] = loop[0] // close the contour
		loops = append(loops, loop)
	}
	return loops
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
