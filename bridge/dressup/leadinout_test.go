// SPDX-License-Identifier: GPL-2.0-only

package dressup

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// profileLoop is a walkLoop-shaped profile pass: clearance rapid, position over the start,
// rapid to the safe plane, straight plunge, a triangular cut loop closing back at the start,
// then a retract. It mirrors gen.walkLoop's output so the dressup is exercised on real shape.
func leadProfileLoop() gcode.Path {
	g := func(name string, p map[string]float64) gcode.Command { return gcode.NewCommand(name, p) }
	return gcode.Path{Commands: []gcode.Command{
		g("G0", map[string]float64{"Z": 5}),
		g("G0", map[string]float64{"X": 0, "Y": 0}), // position over the contour start
		g("G0", map[string]float64{"Z": 1}),         // safe plane
		g("G1", map[string]float64{"Z": -2, "F": 100}),
		g("G1", map[string]float64{"X": 10, "Y": 0, "F": 200}),
		g("G1", map[string]float64{"X": 10, "Y": 10, "F": 200}),
		g("G1", map[string]float64{"X": 0, "Y": 0, "F": 200}), // close the loop
		g("G0", map[string]float64{"Z": 5}),                   // retract
	}}
}

// countArcs counts the G2/G3 moves in a path.
func countArcs(p gcode.Path) int {
	n := 0
	for _, c := range p.Commands {
		if c.Name == "G2" || c.Name == "G3" {
			n++
		}
	}
	return n
}

// TestApplyLeadInOutWrapsSequence wraps the cut sequence with a lead-in and a lead-out arc and
// relocates the plunge clear of the contour start.
func TestApplyLeadInOutWrapsSequence(t *testing.T) {
	r := 3.0
	out := ApplyLeadInOut(leadProfileLoop(), LeadInOutParams{Radius: r, Side: SideLeft})

	if got := countArcs(out); got != 2 {
		t.Fatalf("expected a lead-in + lead-out arc (2), got %d arcs", got)
	}

	// the positioning rapid must be moved off the contour start (0,0) onto the lead-in start
	var rapid gcode.Command
	for _, c := range out.Commands {
		if c.Name == "G0" {
			if _, hasX := c.Params["X"]; hasX {
				rapid = c
				break
			}
		}
	}
	if rapid.Params["X"] == 0 && rapid.Params["Y"] == 0 {
		t.Errorf("positioning rapid was not relocated off the contour start: %+v", rapid.Params)
	}
	// it must be the lead-in start: a quarter-arc back, radius*sqrt(2) from the contour start
	if d := math.Hypot(rapid.Params["X"], rapid.Params["Y"]); math.Abs(d-r*math.Sqrt2) > 1e-9 {
		t.Errorf("lead-in start distance = %g, want r*sqrt2 = %g", d, r*math.Sqrt2)
	}
}

// TestLeadInArcIsTangent checks the lead-in arc ends at the contour start, at cut depth, with
// its end radius perpendicular to the first cut direction (so the tool eases in tangentially).
func TestLeadInArcIsTangent(t *testing.T) {
	out := ApplyLeadInOut(leadProfileLoop(), LeadInOutParams{Radius: 3, Side: SideLeft})

	var lead gcode.Command
	for _, c := range out.Commands {
		if c.Name == "G2" || c.Name == "G3" {
			lead = c
			break // the first arc is the lead-in
		}
	}
	if lead.Name == "" {
		t.Fatal("no lead-in arc emitted")
	}
	if lead.Params["X"] != 0 || lead.Params["Y"] != 0 || lead.Params["Z"] != -2 {
		t.Errorf("lead-in should end at the contour start (0,0,-2), got %+v", lead.Params)
	}
	// end-radius (end - centre) must be perpendicular to the first cut direction (+X here).
	// centre = begin + (I,J); begin is the relocated rapid start.
	begin := leadInStart(out)
	cx := begin.X + lead.Params["I"]
	cy := begin.Y + lead.Params["J"]
	endRadX, endRadY := 0-cx, 0-cy // contour start minus centre
	if dot := endRadX*1 + endRadY*0; math.Abs(dot) > 1e-9 {
		t.Errorf("lead-in arc not tangent to +X cut: end-radius·d = %g, want 0", dot)
	}
}

// leadInStart finds the relocated positioning rapid (the lead-in arc's begin point).
func leadInStart(p gcode.Path) gcode.Vector3 {
	for _, c := range p.Commands {
		if c.Name == "G0" {
			if x, hasX := c.Params["X"]; hasX {
				return gcode.Vector3{X: x, Y: c.Params["Y"]}
			}
		}
	}
	return gcode.Vector3{}
}

// TestApplyLeadInOutNoOp leaves the path unchanged for zero radius or a plunge with no cut.
func TestApplyLeadInOutNoOp(t *testing.T) {
	in := leadProfileLoop()
	if out := ApplyLeadInOut(in, LeadInOutParams{Radius: 0}); len(out.Commands) != len(in.Commands) {
		t.Error("zero radius must be a no-op")
	}
	bare := gcode.Path{Commands: []gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"X": 0, "Y": 0, "Z": 5}),
		gcode.NewCommand("G1", map[string]float64{"Z": -2}),
	}}
	if out := ApplyLeadInOut(bare, LeadInOutParams{Radius: 3}); len(out.Commands) != len(bare.Commands) {
		t.Error("a plunge with no following cut must stay a straight plunge")
	}
}

// TestApplyLeadInOutRightSide curves the lead to the opposite side (G2 vs G3).
func TestApplyLeadInOutRightSide(t *testing.T) {
	left := ApplyLeadInOut(leadProfileLoop(), LeadInOutParams{Radius: 3, Side: SideLeft})
	right := ApplyLeadInOut(leadProfileLoop(), LeadInOutParams{Radius: 3, Side: SideRight})

	leadName := func(p gcode.Path) string {
		for _, c := range p.Commands {
			if c.Name == "G2" || c.Name == "G3" {
				return c.Name
			}
		}
		return ""
	}
	if leadName(left) == leadName(right) {
		t.Errorf("left and right leads should turn opposite ways, both %s", leadName(left))
	}
}
