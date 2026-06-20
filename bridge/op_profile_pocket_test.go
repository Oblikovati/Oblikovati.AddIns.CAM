// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/geom2d"
)

// millJob builds a one-controller job with a milling tool of the given diameter.
func millJob(diameter float64) *Job {
	j := NewJob()
	j.Tools = []ToolController{{Label: "EM", ToolNumber: 1, VertFeed: 60, HorizFeed: 300, SpindleSpeed: 5000, SpindleDir: "Forward", Tool: ToolBit{ShapeType: "endmill", Diameter: diameter}}}
	return j
}

// squarePoly returns a CCW square [0,s]×[0,s].
func squarePoly(s float64) geom2d.Polygon {
	return geom2d.Polygon{{X: 0, Y: 0}, {X: s, Y: 0}, {X: s, Y: s}, {X: 0, Y: s}}
}

// hasCutMove reports whether the path contains a G1 XY feed move.
func hasCutMove(path gcode.Path) bool {
	for _, c := range path.Commands {
		_, hasX := c.Params["X"]
		if c.Name == "G1" && hasX {
			return true
		}
	}
	return false
}

// TestProfileOpExecute checks a profile op frames a real contour toolpath and reports its
// features.
func TestProfileOpExecute(t *testing.T) {
	op := &ProfileOp{
		OpBase:   OpBase{OpLabel: "Profile", IsActive: true, ClearanceHeight: 15, SafeHeight: 2, StartDepth: 0, FinalDepth: -6},
		Side:     "outside",
		Climb:    true,
		StepDown: 3,
		Boundary: squarePoly(10),
	}
	path, err := op.Execute(millJob(2))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if path.Commands[0].Name != "(Profile)" {
		t.Errorf("first command = %q, want the label comment", path.Commands[0].Name)
	}
	last := path.Commands[len(path.Commands)-1]
	if last.Name != "G0" || last.Params["Z"] != 15 {
		t.Errorf("last command = %+v, want a clearance retract", last)
	}
	if !hasCutMove(path) {
		t.Error("profile produced no cutting moves")
	}
	if !op.Features().Has(FeatureStepDown) || !op.Features().Has(FeatureBaseGeometry) {
		t.Error("profile should advertise step-down + base geometry features")
	}
}

// TestProfileOpErrors covers the missing-boundary and oversized-tool guards.
func TestProfileOpErrors(t *testing.T) {
	noBoundary := &ProfileOp{OpBase: OpBase{OpLabel: "P", IsActive: true}, Side: "outside"}
	if _, err := noBoundary.Execute(millJob(2)); err == nil {
		t.Error("a profile with no boundary must error")
	}
	tooBig := &ProfileOp{OpBase: OpBase{OpLabel: "P", IsActive: true}, Side: "inside", Boundary: squarePoly(10)}
	if _, err := tooBig.Execute(millJob(20)); err == nil {
		t.Error("an oversized tool on an inside profile must error")
	}
}

// TestPocketOpExecute checks a pocket op clears a region with several ring passes.
func TestPocketOpExecute(t *testing.T) {
	op := &PocketOp{
		OpBase:   OpBase{OpLabel: "Pocket", IsActive: true, ClearanceHeight: 15, SafeHeight: 2, StartDepth: 0, FinalDepth: -2},
		StepOver: 0.5,
		Climb:    true,
		Boundary: squarePoly(20),
	}
	path, err := op.Execute(millJob(4))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	plunges := 0
	for _, c := range path.Commands {
		if _, hasZ := c.Params["Z"]; c.Name == "G1" && hasZ {
			plunges++
		}
	}
	if plunges < 2 {
		t.Errorf("pocket should clear with multiple rings, got %d plunge moves", plunges)
	}

	tooSmall := &PocketOp{OpBase: OpBase{OpLabel: "P", IsActive: true}, Boundary: squarePoly(2)}
	if _, err := tooSmall.Execute(millJob(4)); err == nil {
		t.Error("a pocket smaller than the tool must error")
	}
}
