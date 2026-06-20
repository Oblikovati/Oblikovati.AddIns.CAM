// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"encoding/json"
	"strings"
	"testing"

	"oblikovati.org/api/wire"
	"oblikovati.org/cam/bridge/ocl"
)

// comboHost serves everything a combined job needs: the two-hole plate topology + section
// (from recordingHost) plus a tessellation for the surface op.
type comboHost struct{ recordingHost }

func (h *comboHost) Call(method string, req []byte) ([]byte, error) {
	if method == wire.MethodBodyCalculateFacets {
		return json.Marshal(wire.FacetSetResult{
			VertexCount: 4, FacetCount: 2,
			VertexCoordinates: []float64{0, 0, 1, 4, 0, 1, 4, 4, 1, 0, 4, 1},
			VertexIndices:     []int{0, 1, 2, 0, 2, 3},
			IndexCountPerFace: []int{3, 3},
		})
	}
	return h.recordingHost.Call(method, req)
}

// TestRunAllJobs generates one program over drilling + profile + surface and checks it carries
// the three distinct tool changes (drill T2, end mill T1, ball-nose T3).
func TestRunAllJobs(t *testing.T) {
	fs := &fakeSurfacer{rows: [][]ocl.Point3{
		{{X: 0, Y: 0, Z: 1}, {X: 0, Y: 4, Z: 1}},
		{{X: 3, Y: 0, Z: 1}, {X: 3, Y: 4, Z: 1}},
	}}
	res, err := NewEngine(&comboHost{}).WithSurfacer(fs).RunAllJobsOnHost(0)
	if err != nil {
		t.Fatalf("RunAllJobsOnHost: %v", err)
	}
	if !strings.Contains(res.Summary, "3 tools") {
		t.Errorf("summary should report 3 tools, got %q", res.Summary)
	}
	for _, tn := range []string{"T1", "T2", "T3"} {
		if !strings.Contains(res.GCode, "M6 "+tn) {
			t.Errorf("program missing tool change %q:\n%s", tn, firstLines(res.GCode, 40))
		}
	}
}

// TestDistinctTools counts the tool changes a set of ops will emit.
func TestDistinctTools(t *testing.T) {
	ops := []Operation{
		&DrillingOp{OpBase: OpBase{ToolController: 1}},
		&ProfileOp{OpBase: OpBase{ToolController: 0}},
		&SurfaceOp{OpBase: OpBase{ToolController: 2}},
		&ProfileOp{OpBase: OpBase{ToolController: 0}}, // same tool as the first profile
	}
	if got := distinctTools(ops); got != 3 {
		t.Errorf("distinctTools = %d, want 3", got)
	}
}

// firstLines returns the first n lines of s, for test diagnostics.
func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
