// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"encoding/json"
	"testing"

	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// sampleJob builds a job with a tool and one of each persisted operation kind.
func sampleJob() *Job {
	j := NewJob()
	j.PostProcessor = "grbl"
	j.GeometryTolerance = 0.02
	j.Tools = []ToolController{{Label: "T1", ToolNumber: 2, SpindleSpeed: 4000, SpindleDir: "Forward", VertFeed: 80, HorizFeed: 240, Tool: ToolBit{Name: "EM", ShapeType: "endmill", Diameter: 6}}}
	j.Operations = []Operation{
		&DrillingOp{OpBase: OpBase{OpLabel: "Drill", IsActive: true, ToolController: 0, RetractHeight: 12}, PeckDepth: 1.5, Repeat: 2},
		&ProfileOp{OpBase: OpBase{OpLabel: "Prof", IsActive: true, StartDepth: 0, FinalDepth: -5}, Side: "outside", OffsetExtra: 0.2, Climb: true, StepDown: 3},
		&HelixOp{OpBase: OpBase{OpLabel: "Bore", IsActive: false}, HoleRadius: 8, Pitch: 1.2, Direction: "CCW"},
	}
	return j
}

// TestJobMarshalRoundTrip serialises a job and rebuilds it, checking the configuration
// survives (geometry is intentionally not persisted).
func TestJobMarshalRoundTrip(t *testing.T) {
	payload, err := MarshalJob(sampleJob())
	if err != nil {
		t.Fatalf("MarshalJob: %v", err)
	}
	back, err := UnmarshalJob(payload)
	if err != nil {
		t.Fatalf("UnmarshalJob: %v", err)
	}
	if back.PostProcessor != "grbl" || back.GeometryTolerance != 0.02 {
		t.Errorf("job header not preserved: %+v", back)
	}
	if len(back.Tools) != 1 || back.Tools[0].Tool.Diameter != 6 || back.Tools[0].ToolNumber != 2 {
		t.Errorf("tools not preserved: %+v", back.Tools)
	}
	if len(back.Operations) != 3 {
		t.Fatalf("want 3 operations, got %d", len(back.Operations))
	}
	drill, ok := back.Operations[0].(*DrillingOp)
	if !ok || drill.PeckDepth != 1.5 || drill.Repeat != 2 || drill.RetractHeight != 12 {
		t.Errorf("drilling op not preserved: %+v", back.Operations[0])
	}
	prof, ok := back.Operations[1].(*ProfileOp)
	if !ok || prof.Side != "outside" || prof.OffsetExtra != 0.2 || !prof.Climb || prof.FinalDepth != -5 {
		t.Errorf("profile op not preserved: %+v", back.Operations[1])
	}
	helix, ok := back.Operations[2].(*HelixOp)
	if !ok || helix.HoleRadius != 8 || helix.Pitch != 1.2 || helix.Direction != "CCW" || helix.IsActive {
		t.Errorf("helix op not preserved: %+v", back.Operations[2])
	}
}

// persistHost answers documents.list with one active document and round-trips one stored
// attribute value through attributes.set / attributes.get.
type persistHost struct{ stored *types.Variant }

func (h *persistHost) Call(method string, req []byte) ([]byte, error) {
	switch method {
	case wire.MethodDocumentsList:
		return json.Marshal(wire.ListDocumentsResult{Documents: []wire.DocumentInfo{{ID: 7, Name: "Part1", Active: true}}})
	case wire.MethodAttributesSet:
		var a wire.SetAttributeArgs
		if err := json.Unmarshal(req, &a); err != nil {
			return nil, err
		}
		h.stored = &a.Value
		return json.Marshal(wire.AttributeResult{Found: true, Attribute: wire.AttributeInfo{Set: a.Set, Name: a.Name, Value: a.Value}})
	case wire.MethodAttributesGet:
		if h.stored == nil {
			return json.Marshal(wire.AttributeResult{Found: false, Attribute: wire.AttributeInfo{Value: types.StringVariant("")}})
		}
		return json.Marshal(wire.AttributeResult{Found: true, Attribute: wire.AttributeInfo{Set: CAMAttributeSet, Name: CAMJobAttribute, Value: *h.stored}})
	default:
		return []byte("{}"), nil
	}
}

// TestEngineSaveLoadJob persists a job through the host attribute store and reads it back.
func TestEngineSaveLoadJob(t *testing.T) {
	e := NewEngine(&persistHost{})

	// Nothing stored yet.
	if job, err := e.LoadJob(); err != nil || job != nil {
		t.Fatalf("LoadJob before save = (%v, %v), want (nil, nil)", job, err)
	}
	if err := e.SaveJob(sampleJob()); err != nil {
		t.Fatalf("SaveJob: %v", err)
	}
	back, err := e.LoadJob()
	if err != nil {
		t.Fatalf("LoadJob: %v", err)
	}
	if back == nil || len(back.Operations) != 3 || back.PostProcessor != "grbl" {
		t.Errorf("loaded job not round-tripped: %+v", back)
	}
}

// TestSaveJobNoDocument errors when no document is active.
func TestSaveJobNoDocument(t *testing.T) {
	e := NewEngine(&recordingHost{}) // documents.list returns "{}" → no active doc
	if err := e.SaveJob(sampleJob()); err == nil {
		t.Error("SaveJob with no active document must error")
	}
}
