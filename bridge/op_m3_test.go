// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"strings"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// TestHelixOpExecute bores a hole wider than the tool: the path contains descending G2 arcs
// and is framed.
func TestHelixOpExecute(t *testing.T) {
	op := &HelixOp{
		OpBase:     OpBase{OpLabel: "Helix", IsActive: true, ClearanceHeight: 15},
		HoleRadius: 8, Pitch: 1, Direction: "CW",
		Holes: []DrillTarget{{X: 0, Y: 0, Top: 10, Bottom: 0}},
	}
	path, err := op.Execute(millJob(4)) // tool ⌀4 → helix radius 8-2 = 6
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	arcs := 0
	for _, c := range path.Commands {
		if c.Name == "G2" {
			arcs++
		}
	}
	if arcs == 0 {
		t.Error("helix bore should emit G2 arcs")
	}
	if path.Commands[0].Name != "(Helix)" {
		t.Errorf("first command = %q, want the label comment", path.Commands[0].Name)
	}

	// A tool wider than the hole errors.
	tooBig := &HelixOp{OpBase: OpBase{OpLabel: "H", IsActive: true}, HoleRadius: 1, Pitch: 1, Holes: op.Holes}
	if _, err := tooBig.Execute(millJob(4)); err == nil {
		t.Error("a tool wider than the hole must error")
	}
}

// TestMillFaceOpExecute faces a region and frames the toolpath.
func TestMillFaceOpExecute(t *testing.T) {
	op := &MillFaceOp{
		OpBase:   OpBase{OpLabel: "Face", IsActive: true, ClearanceHeight: 15, SafeHeight: 2, StartDepth: 0, FinalDepth: -1},
		StepOver: 0.5,
		Boundary: squarePoly(30),
	}
	path, err := op.Execute(millJob(6))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !hasCutMove(path) {
		t.Error("face milling produced no cutting moves")
	}
	if !op.Features().Has(FeatureStepDown) {
		t.Error("face milling should advertise step-down")
	}

	empty := &MillFaceOp{OpBase: OpBase{OpLabel: "F", IsActive: true}}
	if _, err := empty.Execute(millJob(6)); err == nil {
		t.Error("face milling with no boundary must error")
	}
}

// TestEngraveOpExecute engraves a contour on the tool centre (no compensation): the cut loop
// equals the boundary (area unchanged).
func TestEngraveOpExecute(t *testing.T) {
	op := &EngraveOp{
		OpBase:   OpBase{OpLabel: "Engrave", IsActive: true, ClearanceHeight: 15, SafeHeight: 2, StartDepth: 0, FinalDepth: -0.5},
		Climb:    true,
		Boundary: squarePoly(10),
	}
	path, err := op.Execute(millJob(1))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if a := engraveCutArea(path); a < 99 || a > 101 {
		t.Errorf("engrave cut area = %g, want ~100 (no compensation)", a)
	}
}

// engraveCutArea reconstructs the first cut loop's area from G1 XY moves.
func engraveCutArea(path gcode.Path) float64 {
	var pts []gcode.Vector3
	collecting := false
	for _, c := range path.Commands {
		x, hasX := c.Params["X"]
		y, hasY := c.Params["Y"]
		if c.Name == "G1" && hasX && hasY {
			pts = append(pts, gcode.Vector3{X: x, Y: y})
			collecting = true
		} else if collecting {
			break
		}
	}
	// Shoelace.
	var sum float64
	for i := range pts {
		j := (i + 1) % len(pts)
		sum += pts[i].X*pts[j].Y - pts[j].X*pts[i].Y
	}
	if sum < 0 {
		sum = -sum
	}
	return sum / 2
}

// TestHelixDirectionG3 confirms a CCW helix emits G3 arcs.
func TestHelixDirectionG3(t *testing.T) {
	op := &HelixOp{
		OpBase:     OpBase{OpLabel: "Helix", IsActive: true, ClearanceHeight: 15},
		HoleRadius: 8, Pitch: 1, Direction: "CCW",
		Holes: []DrillTarget{{X: 0, Y: 0, Top: 10, Bottom: 0}},
	}
	path, _ := op.Execute(millJob(4))
	gtext := ""
	for _, c := range path.Commands {
		gtext += c.Name + " "
	}
	if !strings.Contains(gtext, "G3") || strings.Contains(gtext, "G2") {
		t.Errorf("CCW helix should use G3 not G2: %s", gtext)
	}
}
