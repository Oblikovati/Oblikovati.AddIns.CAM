// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/dressup"
)

// TestProfileOpWithDogbone runs a profile op that carries a dogbone dressup and checks the
// dressup added relief moves to the framed toolpath.
func TestProfileOpWithDogbone(t *testing.T) {
	base := OpBase{OpLabel: "Profile", IsActive: true, ClearanceHeight: 15, SafeHeight: 2, StartDepth: 0, FinalDepth: -3}
	plain := &ProfileOp{OpBase: base, Side: "inside", Climb: true, Boundary: squarePoly(20)}
	plainPath, err := plain.Execute(millJob(4))
	if err != nil {
		t.Fatalf("plain Execute: %v", err)
	}

	withBone := base
	withBone.Dressups = []Dressup{NewDogboneDressup(dressup.StyleTBoneH, 2, math.Pi/4, dressup.SideBoth)}
	boned := &ProfileOp{OpBase: withBone, Side: "inside", Climb: true, Boundary: squarePoly(20)}
	bonedPath, err := boned.Execute(millJob(4))
	if err != nil {
		t.Fatalf("dressed Execute: %v", err)
	}
	if len(bonedPath.Commands) <= len(plainPath.Commands) {
		t.Errorf("dogbone dressup added no moves: plain=%d dressed=%d", len(plainPath.Commands), len(bonedPath.Commands))
	}
}

// TestDressupRoundTrip checks an operation's dressup chain survives job serialisation.
func TestDressupRoundTrip(t *testing.T) {
	j := millJob(4)
	j.PostProcessor = "grbl"
	op := &ProfileOp{OpBase: OpBase{OpLabel: "P", IsActive: true, Dressups: []Dressup{
		NewTagsDressup(4, 3, 1),
		NewDogboneDressup(dressup.StyleDogbone, 2.5, math.Pi/4, dressup.SideLeft),
		NewLeadInOutDressup(1.5, dressup.SideRight),
	}}, Side: "outside"}
	j.Operations = []Operation{op}

	payload, err := MarshalJob(j)
	if err != nil {
		t.Fatalf("MarshalJob: %v", err)
	}
	back, err := UnmarshalJob(payload)
	if err != nil {
		t.Fatalf("UnmarshalJob: %v", err)
	}
	rebuilt := back.Operations[0].(*ProfileOp)
	if len(rebuilt.Dressups) != 3 {
		t.Fatalf("want 3 dressups after round-trip, got %d", len(rebuilt.Dressups))
	}
	tags, ok := rebuilt.Dressups[0].(TagsDressup)
	if !ok || tags.Params.Count != 4 || tags.Params.Width != 3 || tags.Params.Height != 1 {
		t.Errorf("tags dressup not preserved: %+v", rebuilt.Dressups[0])
	}
	bone, ok := rebuilt.Dressups[1].(DogboneDressup)
	if !ok || bone.Params.Style != dressup.StyleDogbone || bone.Params.Length != 2.5 || bone.Params.Side != dressup.SideLeft {
		t.Errorf("dogbone dressup not preserved: %+v", rebuilt.Dressups[1])
	}
	lead, ok := rebuilt.Dressups[2].(LeadInOutDressup)
	if !ok || lead.Params.Radius != 1.5 || lead.Params.Side != dressup.SideRight {
		t.Errorf("lead-in/out dressup not preserved: %+v", rebuilt.Dressups[2])
	}
}

// TestProfileOpWithLeadInOut runs a profile op carrying a lead-in/out dressup and checks it
// adds tangential arc moves the plain toolpath lacks.
func TestProfileOpWithLeadInOut(t *testing.T) {
	base := OpBase{OpLabel: "Profile", IsActive: true, ClearanceHeight: 15, SafeHeight: 2, StartDepth: 0, FinalDepth: -3}
	withLead := base
	withLead.Dressups = []Dressup{NewLeadInOutDressup(2, dressup.SideLeft)}
	op := &ProfileOp{OpBase: withLead, Side: "outside", Climb: true, Boundary: squarePoly(20)}
	path, err := op.Execute(millJob(4))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	arcs := 0
	for _, c := range path.Commands {
		if c.Name == "G2" || c.Name == "G3" {
			arcs++
		}
	}
	if arcs == 0 {
		t.Error("lead-in/out dressup added no arc moves")
	}
}
