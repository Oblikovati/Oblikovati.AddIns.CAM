// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"encoding/json"
	"strings"
	"testing"

	"oblikovati.org/api/wire"
)

// selectionHost is a fake host with one selected top face: a 4×4×1 cm plate whose top planar
// face (key "F-top") is selected, sectioned to a 4 cm square outline. It answers the
// face-evaluate calls with the top face's parameter range and its +Z normal.
type selectionHost struct {
	faceEvalInputs [][]float64 // records the inputs of each body.faceEvaluate call
}

func (h *selectionHost) Call(method string, req []byte) ([]byte, error) {
	switch method {
	case wire.MethodModelSelection:
		return json.Marshal(wire.SelectionResult{Count: 1, Kinds: []int{1}, Refs: []string{"F-top"}})
	case wire.MethodModelReferenceKeys:
		return json.Marshal(wire.ReferenceKeysResult{Bodies: []wire.BodyTopology{{Faces: []wire.TopologyRef{
			{Key: "F-top", Kind: "plane", Point: []float64{2, 2, 1}},
			{Key: "F-side", Kind: "plane", Point: []float64{0, 2, 0.5}},
		}}}})
	case wire.MethodBodyFaceEvaluate:
		var a wire.FaceEvaluateArgs
		_ = json.Unmarshal(req, &a)
		h.faceEvalInputs = append(h.faceEvalInputs, a.Inputs)
		res := wire.FaceEvaluateResult{ParamRange: []float64{0, 0, 4, 4}}
		if len(a.Inputs) >= 2 {
			res.Points = []float64{a.Inputs[0], a.Inputs[1], 1}
			res.Normals = []float64{0, 0, 1}
		}
		return json.Marshal(res)
	case wire.MethodBodyRangeBox:
		return json.Marshal(wire.BodyRangeBoxResult{Min: []float64{0, 0, 0}, Max: []float64{4, 4, 1}})
	case wire.MethodBrepSectionWithPlane:
		return json.Marshal(wire.BrepWiresResult{Wires: []wire.WirePolyline{{
			Points: []float64{0, 0, 1, 4, 0, 1, 4, 4, 1, 0, 4, 1, 0, 0, 1}, Closed: true,
		}}})
	default:
		return []byte("{}"), nil
	}
}

// TestSelectedFacePlane resolves the selected top face to its plane: origin at the face's
// representative point and its +Z normal, evaluated at the parameter-range midpoint.
func TestSelectedFacePlane(t *testing.T) {
	h := &selectionHost{}
	plane, ok, err := NewEngine(h).selectedSectionPlane()
	if err != nil || !ok {
		t.Fatalf("selectedSectionPlane = (ok=%v, err=%v), want a plane", ok, err)
	}
	if plane.source != "selected face" {
		t.Errorf("source = %q, want %q", plane.source, "selected face")
	}
	if plane.origin[2] != 1 || plane.normal[2] != 1 {
		t.Errorf("plane origin/normal = %v / %v, want top face at z=1 with +Z normal", plane.origin, plane.normal)
	}
	// the normal must have been evaluated at the (2,2) parameter-range midpoint
	last := h.faceEvalInputs[len(h.faceEvalInputs)-1]
	if len(last) != 2 || last[0] != 2 || last[1] != 2 {
		t.Errorf("normal evaluated at params %v, want the midpoint [2 2]", last)
	}
}

// TestProfileJobUsesSelectedFace runs a profile job with a selected face and checks the
// summary reports the contour came from the selected face (not mid-height).
func TestProfileJobUsesSelectedFace(t *testing.T) {
	res, err := NewEngine(&selectionHost{}).RunProfileJobOnHost(0)
	if err != nil {
		t.Fatalf("RunProfileJobOnHost: %v", err)
	}
	if !strings.Contains(res.Summary, "selected face") {
		t.Errorf("summary = %q, want it to mention the selected face", res.Summary)
	}
}

// TestNoSelectionFallsBackToMidHeight confirms an empty selection sections at mid-height.
func TestNoSelectionFallsBackToMidHeight(t *testing.T) {
	plane, ok, err := NewEngine(&recordingHost{}).selectedSectionPlane()
	if err != nil {
		t.Fatalf("selectedSectionPlane: %v", err)
	}
	if ok {
		t.Errorf("no selection must not yield a face plane, got %+v", plane)
	}
}

// TestSelectedNonPlanarIgnored confirms a selection of only non-planar faces is ignored.
func TestSelectedNonPlanarIgnored(t *testing.T) {
	keys := wire.ReferenceKeysResult{Bodies: []wire.BodyTopology{{Faces: []wire.TopologyRef{
		{Key: "C1", Kind: "cylinder", Point: []float64{1, 1, 0.5}},
	}}}}
	if _, _, _, ok := firstSelectedPlanarFace(keys, []string{"C1"}); ok {
		t.Error("a selected cylindrical face must not be used as a section plane")
	}
}
