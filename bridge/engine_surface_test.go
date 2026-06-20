// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"encoding/json"
	"strings"
	"testing"

	"oblikovati.org/api/wire"
	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/ocl"
)

// surfaceHost is a fake host serving a small two-triangle mesh and a range box for the 3D
// surface flow.
type surfaceHost struct{ recordingHost }

func (h *surfaceHost) Call(method string, req []byte) ([]byte, error) {
	switch method {
	case wire.MethodBodyRangeBox:
		return json.Marshal(wire.BodyRangeBoxResult{Min: []float64{0, 0, 0}, Max: []float64{2, 2, 1}})
	case wire.MethodBodyCalculateFacets:
		// Two triangles (a quad) on the z=1 plane, vertices in cm.
		return json.Marshal(wire.FacetSetResult{
			VertexCount: 4, FacetCount: 2,
			VertexCoordinates: []float64{0, 0, 1, 2, 0, 1, 2, 2, 1, 0, 2, 1},
			VertexIndices:     []int{0, 1, 2, 0, 2, 3},
			IndexCountPerFace: []int{3, 3},
		})
	}
	return h.recordingHost.Call(method, req)
}

// fakeSurfacer records its inputs and returns canned cutter-location rows.
type fakeSurfacer struct {
	tris  int
	lines int
	rows  [][]ocl.Point3
}

func (f *fakeSurfacer) DropCutter(tris []ocl.Triangle, diameter, length, minZ, sampling float64, lines []ocl.ScanLine) ([][]ocl.Point3, error) {
	f.tris, f.lines = len(tris), len(lines)
	return f.rows, nil
}

// TestRunSurface3DJob meshes the body, drops the cutter, and posts the finishing toolpath.
func TestRunSurface3DJob(t *testing.T) {
	fs := &fakeSurfacer{rows: [][]ocl.Point3{
		{{X: 0, Y: 0, Z: 1}, {X: 0, Y: 2, Z: 1}},
		{{X: 1.5, Y: 0, Z: 1}, {X: 1.5, Y: 2, Z: 1}},
	}}
	res, err := NewEngine(&surfaceHost{}).WithSurfacer(fs).RunSurface3DJobOnHost(0)
	if err != nil {
		t.Fatalf("RunSurface3DJobOnHost: %v", err)
	}
	if fs.tris != 2 {
		t.Errorf("surfacer received %d triangles, want 2 (the quad)", fs.tris)
	}
	if fs.lines < 2 {
		t.Errorf("surfacer received %d scan lines, want at least 2 over the 2 mm extent", fs.lines)
	}
	if !strings.Contains(res.Summary, "finished the surface") || res.GCodeLines == 0 {
		t.Errorf("unexpected result: %q, %d lines", res.Summary, res.GCodeLines)
	}
}

// TestRunSurface3DEmptyMesh surfaces an empty tessellation as a job error.
func TestRunSurface3DEmptyMesh(t *testing.T) {
	h := &surfaceHost{}
	h.failOn = wire.MethodBodyCalculateFacets
	if _, err := NewEngine(h).WithSurfacer(&fakeSurfacer{}).RunSurface3DJobOnHost(0); err == nil {
		t.Error("a tessellation failure must fail the surface job")
	}
}

// TestFacetsToTriangles fan-triangulates a quad face and converts cm→mm.
func TestFacetsToTriangles(t *testing.T) {
	f := wire.FacetSetResult{
		VertexCoordinates: []float64{0, 0, 0, 1, 0, 0, 1, 1, 0, 0, 1, 0},
		VertexIndices:     []int{0, 1, 2, 3},
		IndexCountPerFace: []int{4}, // one quad → two triangles
	}
	tris := facetsToTriangles(f)
	if len(tris) != 2 {
		t.Fatalf("quad should fan into 2 triangles, got %d", len(tris))
	}
	if tris[0].B[0] != 10 { // 1 cm → 10 mm
		t.Errorf("cm→mm conversion wrong: %v", tris[0].B)
	}
}

// TestFacetsDropDegenerate filters slivers (zero-area facets) the drop-cutter would abort on.
func TestFacetsDropDegenerate(t *testing.T) {
	f := wire.FacetSetResult{
		// two facets: one real triangle, one degenerate (two coincident corners).
		VertexCoordinates: []float64{0, 0, 0, 1, 0, 0, 0, 1, 0},
		VertexIndices:     []int{0, 1, 2, 0, 1, 1},
		IndexCountPerFace: []int{3, 3},
	}
	if tris := facetsToTriangles(f); len(tris) != 1 {
		t.Errorf("degenerate facet must be dropped: got %d triangles, want 1", len(tris))
	}
	if degenerateTriangle([3]float64{0, 0, 0}, [3]float64{1, 0, 0}, [3]float64{2, 0, 0}) == false {
		t.Error("collinear (zero-area) triangle must be flagged degenerate")
	}
}

// TestScanLinesCoverExtent steps scan lines across the stock X extent.
func TestScanLinesCoverExtent(t *testing.T) {
	stock := Stock{Min: vec(0, 0, 0), Max: vec(10, 6, 2)}
	lines := scanLines(stock, 2)
	if len(lines) != 6 { // x = 0,2,4,6,8,10
		t.Fatalf("want 6 scan lines over a 10 mm extent at 2 mm step, got %d", len(lines))
	}
	if lines[0].Y0 != 0 || lines[0].Y1 != 6 {
		t.Errorf("scan line should span the Y extent, got %+v", lines[0])
	}
	if scanLines(stock, 0) != nil {
		t.Error("non-positive step must yield no lines")
	}
}

// TestSurfaceOpRoundTrip checks a 3D surface op's parameters survive job serialisation (its
// geometry rows are re-resolved, not persisted).
func TestSurfaceOpRoundTrip(t *testing.T) {
	j := NewJob()
	j.PostProcessor = "grbl"
	j.Operations = []Operation{&SurfaceOp{
		OpBase:   OpBase{OpLabel: "Surf", IsActive: true},
		StepOver: 1.5, Sampling: 0.8, Zigzag: true,
		Rows: [][]gcode.Vector3{{{X: 0}, {X: 1}}}, // must NOT persist
	}}
	payload, err := MarshalJob(j)
	if err != nil {
		t.Fatalf("MarshalJob: %v", err)
	}
	back, err := UnmarshalJob(payload)
	if err != nil {
		t.Fatalf("UnmarshalJob: %v", err)
	}
	op, ok := back.Operations[0].(*SurfaceOp)
	if !ok || op.StepOver != 1.5 || op.Sampling != 0.8 || !op.Zigzag {
		t.Fatalf("surface op not preserved: %+v", back.Operations[0])
	}
	if len(op.Rows) != 0 {
		t.Errorf("drop-cutter rows must not persist, got %d", len(op.Rows))
	}
}

// vec is a Vector3 helper for tests.
func vec(x, y, z float64) gcode.Vector3 { return gcode.Vector3{X: x, Y: y, Z: z} }
