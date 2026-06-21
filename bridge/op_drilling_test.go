// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"oblikovati.org/api/wire"
)

// tourTravel sums the XY rapid travel of an ordered hole list.
func tourTravel(holes []DrillTarget) float64 {
	total := 0.0
	for i := 1; i < len(holes); i++ {
		total += math.Hypot(holes[i].X-holes[i-1].X, holes[i].Y-holes[i-1].Y)
	}
	return total
}

// TestOrderedHolesShortensTravel checks the nearest-neighbour tour beats a naive row-by-row
// (Y-then-X) order on a scattered pattern, and is deterministic (same order on a reshuffle).
func TestOrderedHolesShortensTravel(t *testing.T) {
	// A 3×3 grid: row-major Y-then-X snakes back across the full width between rows; the tour
	// should boustrophedon instead and travel less.
	var grid []DrillTarget
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			grid = append(grid, DrillTarget{X: float64(x) * 10, Y: float64(y) * 10})
		}
	}
	rowMajor := append([]DrillTarget(nil), grid...) // already in Y-then-X order
	tour := orderedHoles(grid)
	if tourTravel(tour) >= tourTravel(rowMajor) {
		t.Errorf("tour travel %g should beat row-major %g", tourTravel(tour), tourTravel(rowMajor))
	}
	// Deterministic: reversing the input yields the same tour.
	reversed := make([]DrillTarget, len(grid))
	for i := range grid {
		reversed[i] = grid[len(grid)-1-i]
	}
	a, b := orderedHoles(grid), orderedHoles(reversed)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("tour not deterministic at %d: %+v vs %+v", i, a[i], b[i])
		}
	}
}

// drillJob builds a one-controller job for exercising the drilling op.
func drillJob() *Job {
	j := NewJob()
	j.Tools = []ToolController{{Label: "T1", ToolNumber: 1, VertFeed: 100, SpindleSpeed: 2000, SpindleDir: "Forward", Tool: ToolBit{ShapeType: "drill", Diameter: 6}}}
	return j
}

// TestDrillingExecute checks the full drilling envelope: label comment, lead-in rapid to
// clearance, a rapid-over + canned cycle per hole carrying the controller's plunge feed and
// the R plane, a G80 cancel, and the trailing clearance retract.
func TestDrillingExecute(t *testing.T) {
	op := &DrillingOp{
		OpBase: OpBase{OpLabel: "Drilling", IsActive: true, ToolController: 0, ClearanceHeight: 15, RetractHeight: 12},
		Holes: []DrillTarget{
			{X: 5, Y: 5, Top: 10, Bottom: 0},
			{X: 1, Y: 1, Top: 10, Bottom: 0},
		},
	}
	path, err := op.Execute(drillJob())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	names := make([]string, len(path.Commands))
	for i, c := range path.Commands {
		names[i] = c.Name
	}
	// (Drilling), G0(Z15), [G0(XY) G81]x2 sorted by Y then X → (1,1) before (5,5), G80, G0(Z15).
	want := []string{"(Drilling)", "G0", "G0", "G81", "G0", "G81", "G80", "G0"}
	if len(names) != len(want) {
		t.Fatalf("command names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("command[%d] = %q, want %q (full: %v)", i, names[i], want[i], names)
		}
	}

	// First cycle is the (1,1) hole (sorted first); verify its addressed parameters.
	cycle := path.Commands[3]
	for addr, w := range map[string]float64{"X": 1, "Y": 1, "Z": 0, "R": 12, "F": 100} {
		if got := cycle.Params[addr]; got != w {
			t.Errorf("cycle param %s = %g, want %g", addr, got, w)
		}
	}
}

// TestDrillingErrors covers the missing-tool and empty-holes guards.
func TestDrillingErrors(t *testing.T) {
	noHoles := &DrillingOp{OpBase: OpBase{OpLabel: "D", IsActive: true, ToolController: 0}}
	if _, err := noHoles.Execute(drillJob()); err == nil {
		t.Error("empty holes must error")
	}
	badTool := &DrillingOp{OpBase: OpBase{OpLabel: "D", ToolController: 9}, Holes: []DrillTarget{{Top: 10}}}
	if _, err := badTool.Execute(drillJob()); err == nil {
		t.Error("missing tool controller must error")
	}
}

// TestDetectDrillTargets confirms cylindrical faces become coaxial-deduped holes spanning
// the body extent, with cm→mm conversion, and non-cylinder faces ignored.
func TestDetectDrillTargets(t *testing.T) {
	refs := wire.ReferenceKeysResult{Bodies: []wire.BodyTopology{{
		Faces: []wire.TopologyRef{
			{Key: "f1", Kind: "plane", Point: []float64{0, 0, 1}},
			{Key: "f2", Kind: "cylinder", Point: []float64{1.0, 2.0, 0.5}}, // cm
			{Key: "f3", Kind: "cylinder", Point: []float64{1.0, 2.0, 0.5}}, // coaxial dup
			{Key: "f4", Kind: "cylinder", Point: []float64{3.0, 0.0, 0.5}}, // cm
		},
	}}}
	rbox := wire.BodyRangeBoxResult{Min: []float64{0, 0, 0}, Max: []float64{5, 5, 1}} // cm

	targets, err := DetectDrillTargets(refs, rbox, 0)
	if err != nil {
		t.Fatalf("DetectDrillTargets: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("want 2 holes (coaxial deduped), got %d: %+v", len(targets), targets)
	}
	// Sorted by Y then X: (3,0)→(30,0) before (1,2)→(10,20). Through hole spans 0..10mm.
	first := targets[0]
	if first.X != 30 || first.Y != 0 || first.Top != 10 || first.Bottom != 0 {
		t.Errorf("first hole = %+v, want X30 Y0 Top10 Bottom0", first)
	}
}
