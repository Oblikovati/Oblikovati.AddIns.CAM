// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"strings"
	"testing"

	"oblikovati.org/api/wire"
	"oblikovati.org/cam/bridge/ocl"
)

// coneSurfacer is a fake Surfacer that ignores the mesh and returns a cone height field
// (tip Z = peak − radius) sampled on each requested scan line, so the engine has a real curved
// surface to contour.
type coneSurfacer struct{ peak float64 }

func (c coneSurfacer) DropCutter(_ []ocl.Triangle, _, _, _, sampling float64, lines []ocl.ScanLine) ([][]ocl.Point3, error) {
	rows := make([][]ocl.Point3, len(lines))
	for i, ln := range lines {
		var row []ocl.Point3
		for y := ln.Y0; y <= ln.Y1+1e-9; y += sampling {
			z := c.peak - math.Hypot(ln.X0, y)
			row = append(row, ocl.Point3{X: ln.X0, Y: y, Z: z})
		}
		rows[i] = row
	}
	return rows, nil
}

// TestRunWaterlineJob contours a cone surface into several constant-Z levels and posts them.
func TestRunWaterlineJob(t *testing.T) {
	res, err := NewEngine(&surfaceHost{}).WithSurfacer(coneSurfacer{peak: 20}).RunWaterlineJobOnHost(0)
	if err != nil {
		t.Fatalf("RunWaterlineJobOnHost: %v", err)
	}
	if !strings.Contains(res.Summary, "waterline-finished") {
		t.Errorf("summary = %q, want it to mention waterline", res.Summary)
	}
	if !strings.Contains(res.Summary, "levels") || res.GCodeLines == 0 {
		t.Errorf("waterline produced no levels / no G-code: %q (%d lines)", res.Summary, res.GCodeLines)
	}
}

// TestWaterlineUsesOffsetSurface checks the waterline op derives its tool-centre contours from the
// EXACT offset surface (brep.offsetFaces, then sectioning) rather than the drop-cutter heightfield.
func TestWaterlineUsesOffsetSurface(t *testing.T) {
	h := &surfaceHost{}
	res, err := NewEngine(h).WithSurfacer(coneSurfacer{peak: 20}).RunWaterlineJobOnHost(0)
	if err != nil {
		t.Fatalf("RunWaterlineJobOnHost: %v", err)
	}
	if !h.called(wire.MethodBrepOffsetFaces) {
		t.Error("waterline should offset the part surface via brep.offsetFaces")
	}
	if res.GCodeLines == 0 {
		t.Error("waterline from the offset surface produced no toolpath")
	}
}

// TestHeightfieldFromRows resamples scan-line rows onto a regular grid and interpolates Z.
func TestHeightfieldFromRows(t *testing.T) {
	rows := [][]ocl.Point3{
		{{X: 0, Y: 0, Z: 1}, {X: 0, Y: 2, Z: 3}}, // Z rises 1→3 over Y 0→2
		{{X: 1, Y: 0, Z: 2}, {X: 1, Y: 2, Z: 2}},
	}
	hf := heightfieldFromRows(rows, 0, 0, 1, 3) // ny=3 → Y = 0,1,2
	if hf.NX != 2 || hf.NY != 3 {
		t.Fatalf("grid dims = %d×%d, want 2×3", hf.NX, hf.NY)
	}
	if got := hf.Z[0*3+1]; math.Abs(got-2) > 1e-9 { // row0 at Y=1 interpolates to 2
		t.Errorf("interpolated Z at (0,Y=1) = %g, want 2", got)
	}
}

// TestInterpRowZOutOfRange returns NaN past the sampled span.
func TestInterpRowZOutOfRange(t *testing.T) {
	row := []ocl.Point3{{Y: 0, Z: 1}, {Y: 2, Z: 3}}
	if !math.IsNaN(interpRowZ(row, 5)) {
		t.Error("Y beyond the row must interpolate to NaN")
	}
	if math.IsNaN(interpRowZ(row, 1)) {
		t.Error("Y inside the row must interpolate to a value")
	}
}

// TestWaterlineOpRoundTrip checks a waterline op's parameters survive serialisation.
func TestWaterlineOpRoundTrip(t *testing.T) {
	j := NewJob()
	j.PostProcessor = "grbl"
	j.Operations = []Operation{&WaterlineOp{OpBase: OpBase{OpLabel: "WL", IsActive: true}, StepOver: 1, StepDown: 0.5}}
	payload, err := MarshalJob(j)
	if err != nil {
		t.Fatalf("MarshalJob: %v", err)
	}
	back, err := UnmarshalJob(payload)
	if err != nil {
		t.Fatalf("UnmarshalJob: %v", err)
	}
	op, ok := back.Operations[0].(*WaterlineOp)
	if !ok || op.StepOver != 1 || op.StepDown != 0.5 {
		t.Fatalf("waterline op not preserved: %+v", back.Operations[0])
	}
	if len(op.Levels) != 0 {
		t.Errorf("level contours must not persist, got %d", len(op.Levels))
	}
}

// TestWaterlineExecuteNeedsLevels errors without resolved levels.
func TestWaterlineExecuteNeedsLevels(t *testing.T) {
	op := &WaterlineOp{OpBase: OpBase{OpLabel: "WL", IsActive: true}}
	if _, err := op.Execute(millJob(6)); err == nil {
		t.Error("a waterline op with no levels must error")
	}
}
