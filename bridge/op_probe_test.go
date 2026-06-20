// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
)

// sampleProbe is a three-touch corner cycle (Z + two edges).
func sampleProbe() []gen.ProbePoint {
	return []gen.ProbePoint{
		{Approach: gcode.Vector3{X: 5, Y: 5, Z: 2}, Target: gcode.Vector3{X: 5, Y: 5, Z: -5}},
		{Approach: gcode.Vector3{X: -5, Y: 5, Z: -2}, Target: gcode.Vector3{X: 3, Y: 5, Z: -2}},
		{Approach: gcode.Vector3{X: 5, Y: -5, Z: -2}, Target: gcode.Vector3{X: 5, Y: 3, Z: -2}},
	}
}

// TestProbeOpExecute checks a probe op frames G38.2 moves and reports the Probe kind, cutting no
// material (no G1 feed moves).
func TestProbeOpExecute(t *testing.T) {
	op := &ProbeOp{OpBase: OpBase{OpLabel: "Probe", IsActive: true, ClearanceHeight: 15}, ProbeFeed: 40, Points: sampleProbe()}
	path, err := op.Execute(NewJob())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	probes := 0
	for _, c := range path.Commands {
		switch c.Name {
		case "G38.2":
			probes++
			if c.Params["F"] != 40 {
				t.Errorf("probe feed = %g, want 40", c.Params["F"])
			}
		case "G1":
			t.Errorf("a probe op should cut nothing, found a G1: %+v", c.Params)
		}
	}
	if probes != 3 {
		t.Errorf("got %d probe moves, want 3", probes)
	}
	if operationKind(op) != "Probe" {
		t.Errorf("operationKind = %q, want Probe", operationKind(op))
	}

	noPoints := &ProbeOp{OpBase: OpBase{OpLabel: "P", IsActive: true}}
	if _, err := noPoints.Execute(NewJob()); err == nil {
		t.Error("a probe with no points must error")
	}
}

// TestProbeParameters exercises the editable probe parameter and the default feed.
func TestProbeParameters(t *testing.T) {
	op := &ProbeOp{OpBase: OpBase{OpLabel: "Probe"}}
	if op.feedRate() != defaultProbeFeed {
		t.Errorf("unset probe feed = %g, want the default %g", op.feedRate(), defaultProbeFeed)
	}
	op.SetParameter("probeFeed", "80")
	if op.ProbeFeed != 80 || op.feedRate() != 80 {
		t.Errorf("probe feed not applied: %+v", op)
	}
	if !op.SetParameter("label", "Touch-off") { // falls through to the shared depth params
		t.Error("name parameter should be handled by the base")
	}
	if len(op.Parameters()) == 0 {
		t.Error("probe should expose parameters")
	}
}

// TestProbeOpRoundTrip checks the probe op (feed) survives job serialisation.
func TestProbeOpRoundTrip(t *testing.T) {
	j := NewJob()
	j.PostProcessor = "grbl"
	j.Operations = []Operation{&ProbeOp{OpBase: OpBase{OpLabel: "Probe", IsActive: true}, ProbeFeed: 75, WorkOffset: 3}}
	payload, err := MarshalJob(j)
	if err != nil {
		t.Fatalf("MarshalJob: %v", err)
	}
	back, err := UnmarshalJob(payload)
	if err != nil {
		t.Fatalf("UnmarshalJob: %v", err)
	}
	op, ok := back.Operations[0].(*ProbeOp)
	if !ok || op.ProbeFeed != 75 || op.WorkOffset != 3 {
		t.Errorf("probe op not preserved: %+v", back.Operations[0])
	}
}

// TestBoreProbePoints checks the engine builds four outward wall probes per hole, all starting
// from the hole centre at mid depth.
func TestBoreProbePoints(t *testing.T) {
	holes := []DrillTarget{{X: 10, Y: 20, Top: 0, Bottom: -10}}
	pts := boreProbePoints(holes)
	if len(pts) != 4 {
		t.Fatalf("got %d probe points for one hole, want 4", len(pts))
	}
	for _, p := range pts {
		if p.Approach.X != 10 || p.Approach.Y != 20 || p.Approach.Z != -5 {
			t.Errorf("bore probe should approach from the hole centre (10,20,-5), got %+v", p.Approach)
		}
	}
	// the four targets must spread to +X, −X, +Y, −Y of the centre.
	dirs := map[string]bool{}
	for _, p := range pts {
		switch {
		case p.Target.X > 10:
			dirs["+X"] = true
		case p.Target.X < 10:
			dirs["-X"] = true
		case p.Target.Y > 20:
			dirs["+Y"] = true
		case p.Target.Y < 20:
			dirs["-Y"] = true
		}
	}
	if len(dirs) != 4 {
		t.Errorf("bore probe should cover all four directions, got %v", dirs)
	}
}

// TestBossProbePoints checks the engine builds four inward wall probes around the outline, each
// approaching from outside its bounding box and aimed at the footprint centre.
func TestBossProbePoints(t *testing.T) {
	boundary := squarePoly(20) // 0..20 in X and Y → centre (10,10)
	stock := Stock{Min: gcode.Vector3{X: -5, Y: -5, Z: -10}, Max: gcode.Vector3{X: 25, Y: 25, Z: 0}}
	pts := bossProbePoints(boundary, stock)
	if len(pts) != 4 {
		t.Fatalf("got %d probe points, want 4", len(pts))
	}
	for _, p := range pts {
		if p.Target.X != 10 || p.Target.Y != 10 {
			t.Errorf("boss probe should aim at the footprint centre (10,10), got target %+v", p.Target)
		}
	}
	// the +X probe approaches from beyond the +X wall (x > 20) and probes inward (target x < approach).
	if pts[0].Approach.X <= 20 || pts[0].Target.X >= pts[0].Approach.X {
		t.Errorf("+X boss probe should approach outside the wall and probe inward: %+v", pts[0])
	}
}

// TestCornerProbePoints checks the engine builds a Z touch-off plus two edge probes from the
// stock bounds.
func TestCornerProbePoints(t *testing.T) {
	stock := Stock{Min: gcode.Vector3{X: 0, Y: 0, Z: -10}, Max: gcode.Vector3{X: 40, Y: 30, Z: 0}}
	pts := cornerProbePoints(stock)
	if len(pts) != 3 {
		t.Fatalf("got %d probe points, want 3", len(pts))
	}
	// the first is a pure-Z drop (approach above the top, target below it).
	if pts[0].Approach.Z <= stock.Max.Z || pts[0].Target.Z != stock.Min.Z {
		t.Errorf("Z touch-off = %+v, want an approach above the top and a target below", pts[0])
	}
	// the second probes toward +X across the −X face.
	if pts[1].Target.X <= pts[1].Approach.X {
		t.Errorf("X edge probe should move toward +X: %+v", pts[1])
	}
}
