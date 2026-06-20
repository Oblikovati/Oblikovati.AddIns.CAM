// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// TestAddAndClearDressups adds tabs and a dogbone to the selected operation, then clears them.
func TestAddAndClearDressups(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.lastJob = twoOpJob()
	e.editingOp = 0

	if _, err := e.addTabsAction(); err != nil {
		t.Fatalf("addTabsAction: %v", err)
	}
	if _, err := e.addDogboneAction(); err != nil {
		t.Fatalf("addDogboneAction: %v", err)
	}
	prof := e.lastJob.Operations[0].(*ProfileOp)
	if len(prof.Dressups) != 2 {
		t.Fatalf("want 2 dressups after adding tabs + dogbone, got %d", len(prof.Dressups))
	}
	if _, ok := prof.Dressups[0].(TagsDressup); !ok {
		t.Errorf("first dressup should be tabs, got %T", prof.Dressups[0])
	}
	if _, ok := prof.Dressups[1].(DogboneDressup); !ok {
		t.Errorf("second dressup should be dogbone, got %T", prof.Dressups[1])
	}

	if _, err := e.clearDressupsAction(); err != nil {
		t.Fatalf("clearDressupsAction: %v", err)
	}
	if len(prof.Dressups) != 0 {
		t.Errorf("clear should remove all dressups, got %d", len(prof.Dressups))
	}
}

// TestDressupAppliesToToolpath confirms an added tabs dressup changes the generated path.
func TestDressupAppliesToToolpath(t *testing.T) {
	base := OpBase{OpLabel: "Profile", IsActive: true, ClearanceHeight: 15, FinalDepth: -3}
	op := &ProfileOp{OpBase: base, Side: "inside", Climb: true, Boundary: squarePoly(40)}
	plain, err := op.Execute(millJob(4))
	if err != nil {
		t.Fatalf("plain Execute: %v", err)
	}
	op.AppendDressup(NewTagsDressup(defaultTabCount, defaultTabWidth, defaultTabHeight))
	dressed, err := op.Execute(millJob(4))
	if err != nil {
		t.Fatalf("dressed Execute: %v", err)
	}
	// holding tabs lift some cutting moves above the cut depth: the highest cut Z must rise.
	if maxCutZ(dressed) <= maxCutZ(plain) {
		t.Errorf("holding tabs should lift the tool over tabs: plain maxZ=%g dressed maxZ=%g", maxCutZ(plain), maxCutZ(dressed))
	}
}

// maxCutZ returns the highest Z among the path's G1 moves (tabs lift it above the cut depth).
func maxCutZ(path gcode.Path) float64 {
	top := math.Inf(-1)
	for _, c := range path.Commands {
		if c.Name == "G1" {
			if z, ok := c.Params["Z"]; ok {
				top = math.Max(top, z)
			}
		}
	}
	return top
}

// TestAddRampDressup adds a ramp-entry dressup and checks it round-trips through persistence.
func TestAddRampDressup(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.lastJob = twoOpJob()
	e.editingOp = 0
	if _, err := e.addRampAction(); err != nil {
		t.Fatalf("addRampAction: %v", err)
	}
	prof := e.lastJob.Operations[0].(*ProfileOp)
	if len(prof.Dressups) != 1 {
		t.Fatalf("want 1 dressup, got %d", len(prof.Dressups))
	}
	if _, ok := prof.Dressups[0].(RampDressup); !ok {
		t.Errorf("dressup should be a ramp, got %T", prof.Dressups[0])
	}
	// round-trip the job and confirm the ramp survives
	j := NewJob()
	j.Operations = []Operation{prof}
	payload, _ := MarshalJob(j)
	back, err := UnmarshalJob(payload)
	if err != nil {
		t.Fatalf("UnmarshalJob: %v", err)
	}
	if r, ok := back.Operations[0].(*ProfileOp).Dressups[0].(RampDressup); !ok || r.Params.Length != defaultRampLength {
		t.Errorf("ramp dressup did not round-trip: %+v", back.Operations[0])
	}
}

// TestAddLeadInOutDressup adds a lead-in/out dressup and checks it round-trips through persistence.
func TestAddLeadInOutDressup(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.lastJob = twoOpJob()
	e.editingOp = 0
	if _, err := e.addLeadInOutAction(); err != nil {
		t.Fatalf("addLeadInOutAction: %v", err)
	}
	prof := e.lastJob.Operations[0].(*ProfileOp)
	if len(prof.Dressups) != 1 {
		t.Fatalf("want 1 dressup, got %d", len(prof.Dressups))
	}
	if _, ok := prof.Dressups[0].(LeadInOutDressup); !ok {
		t.Errorf("dressup should be a lead-in/out, got %T", prof.Dressups[0])
	}
	j := NewJob()
	j.Operations = []Operation{prof}
	payload, _ := MarshalJob(j)
	back, err := UnmarshalJob(payload)
	if err != nil {
		t.Fatalf("UnmarshalJob: %v", err)
	}
	if l, ok := back.Operations[0].(*ProfileOp).Dressups[0].(LeadInOutDressup); !ok || l.Params.Radius != defaultLeadRadius {
		t.Errorf("lead-in/out dressup did not round-trip: %+v", back.Operations[0])
	}
}
