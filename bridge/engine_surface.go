// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/api/wire"
	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/ocl"
)

// 3D surface-finishing defaults (millimetres).
const (
	surfStepOver     = 1.5  // distance between parallel passes
	surfSampling     = 0.8  // point spacing along a pass
	ballCutterLength = 20.0 // ball-nose cutter length handed to the drop-cutter
	surfaceFacetTol  = 0.01 // tessellation tolerance (cm) for the surface mesh
)

// RunSurface3DJobOnHost finishes the body's top surface with a ball-nose end mill: it meshes
// the part, drops the cutter along parallel scan lines (OpenCAMLib) so each pass rides the
// surface, then posts the finishing toolpath and overlays it.
func (e *Engine) RunSurface3DJobOnHost(bodyIndex int) (*JobResult, error) {
	tris, stock, err := e.meshAndStock(bodyIndex)
	if err != nil {
		return nil, err
	}
	job := e.newMillJob(bodyIndex, stock)
	job.Tools[0].Tool.ShapeType = "ballend" // finishing uses a ball-nose cutter
	diameter := job.Tools[0].Tool.Diameter

	lines := scanLines(stock, surfStepOver)
	rows, err := e.surfacer.DropCutter(tris, diameter, ballCutterLength, stock.BottomZ(), surfSampling, lines)
	if err != nil {
		return nil, fmt.Errorf("drop-cutter over %d triangles / %d passes: %w", len(tris), len(lines), err)
	}
	clRows := toVectorRows(rows)
	job.Operations = []Operation{&SurfaceOp{
		OpBase:   e.millEnvelope("Surface", stock),
		StepOver: surfStepOver, Sampling: surfSampling, Zigzag: true, Rows: clRows,
	}}
	return e.postPreviewResult(job, fmt.Sprintf("finished the surface (%d passes)", countRows(clRows)))
}

// meshAndStock reads the body's range box (for stock) and its tessellation (for the surface
// mesh), returning the triangles in millimetres.
func (e *Engine) meshAndStock(bodyIndex int) ([]ocl.Triangle, Stock, error) {
	rbox, err := e.api.Body().RangeBox(wire.BodyRangeBoxArgs{BodyIndex: bodyIndex, Precise: true})
	if err != nil {
		return nil, Stock{}, fmt.Errorf("read range box of body %d: %w", bodyIndex, err)
	}
	if len(rbox.Min) < 3 || len(rbox.Max) < 3 {
		return nil, Stock{}, fmt.Errorf("body %d has no extent", bodyIndex)
	}
	stock := StockFromRangeBox(rbox.Min, rbox.Max)
	facets, err := e.api.Body().CalculateFacets(wire.CalculateFacetsArgs{BodyIndex: bodyIndex, Tolerance: surfaceFacetTol})
	if err != nil {
		return nil, Stock{}, fmt.Errorf("tessellate body %d: %w", bodyIndex, err)
	}
	tris := facetsToTriangles(facets)
	if len(tris) == 0 {
		return nil, Stock{}, fmt.Errorf("body %d produced no facets to surface", bodyIndex)
	}
	return tris, stock, nil
}

// facetsToTriangles converts a host facet set (vertices in cm, fan-triangulating any polygon
// face) into drop-cutter triangles in millimetres.
func facetsToTriangles(f wire.FacetSetResult) []ocl.Triangle {
	v := f.VertexCoordinates
	pt := func(idx int) [3]float64 {
		b := idx * 3
		if idx < 0 || b+2 >= len(v) {
			return [3]float64{}
		}
		return [3]float64{v[b] * cmToMM, v[b+1] * cmToMM, v[b+2] * cmToMM}
	}
	counts := f.IndexCountPerFace
	if len(counts) == 0 { // no per-face counts → assume a flat triangle list
		counts = triCounts(len(f.VertexIndices) / 3)
	}
	var tris []ocl.Triangle
	pos := 0
	for _, c := range counts {
		if c >= 3 && pos+c <= len(f.VertexIndices) {
			a := pt(f.VertexIndices[pos])
			for k := 1; k+1 < c; k++ { // triangle fan
				b, cc := pt(f.VertexIndices[pos+k]), pt(f.VertexIndices[pos+k+1])
				if !degenerateTriangle(a, b, cc) {
					tris = append(tris, ocl.Triangle{A: a, B: b, C: cc})
				}
			}
		}
		pos += c
	}
	return tris
}

// degenerateTriangle reports whether a facet has a zero-length edge or is collinear (zero
// area). The drop-cutter library rejects such triangles, so they are dropped here rather than
// handed to it — tessellation routinely emits a few slivers.
func degenerateTriangle(a, b, c [3]float64) bool {
	const eps = 1e-7 // mm
	ab, ac := sub3(b, a), sub3(c, a)
	cr := cross3(ab, ac)
	area2 := cr[0]*cr[0] + cr[1]*cr[1] + cr[2]*cr[2]
	return dist2(a, b) < eps || dist2(b, c) < eps || dist2(a, c) < eps || area2 < eps
}

// sub3 returns a − b.
func sub3(a, b [3]float64) [3]float64 { return [3]float64{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }

// cross3 returns a × b.
func cross3(a, b [3]float64) [3]float64 {
	return [3]float64{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}

// dist2 returns the squared distance between a and b.
func dist2(a, b [3]float64) float64 {
	d := sub3(a, b)
	return d[0]*d[0] + d[1]*d[1] + d[2]*d[2]
}

// triCounts returns n threes — the per-face index counts for a flat triangle list.
func triCounts(n int) []int {
	counts := make([]int, n)
	for i := range counts {
		counts[i] = 3
	}
	return counts
}

// scanLines builds parallel Y-spanning scan lines across the stock's X extent, stepped by
// stepOver — the lines the drop-cutter rides down onto the surface.
func scanLines(stock Stock, stepOver float64) []ocl.ScanLine {
	if stepOver <= 0 {
		return nil
	}
	var lines []ocl.ScanLine
	for x := stock.Min.X; x <= stock.Max.X+1e-9; x += stepOver {
		lines = append(lines, ocl.ScanLine{X0: x, Y0: stock.Min.Y, X1: x, Y1: stock.Max.Y})
	}
	return lines
}

// toVectorRows converts drop-cutter point rows into the generator's vector rows.
func toVectorRows(rows [][]ocl.Point3) [][]gcode.Vector3 {
	out := make([][]gcode.Vector3, len(rows))
	for i, row := range rows {
		vs := make([]gcode.Vector3, len(row))
		for j, p := range row {
			vs[j] = gcode.Vector3{X: p.X, Y: p.Y, Z: p.Z}
		}
		out[i] = vs
	}
	return out
}

// countRows counts the rows with enough points to cut (≥2).
func countRows(rows [][]gcode.Vector3) int {
	n := 0
	for _, r := range rows {
		if len(r) >= 2 {
			n++
		}
	}
	return n
}
