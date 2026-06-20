// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/api/wire"
)

// sectionPlane is the plane a milling op slices the body at to extract its driving contour:
// the origin (cm) and unit normal, plus how it was chosen for reporting. A horizontal
// mid-height plane is the default; a selected planar face overrides it so a user can drive
// the profile/pocket from a picked face.
type sectionPlane struct {
	origin []float64 // cm
	normal []float64 // unit vector
	source string    // "selected face" | "mid-height"
}

// sectionPlaneFor returns the plane to contour the body at: the plane of a selected planar
// face when one is picked, otherwise a horizontal plane at the body's mid-height.
func (e *Engine) sectionPlaneFor(bodyIndex int, rbox wire.BodyRangeBoxResult) (sectionPlane, error) {
	plane, ok, err := e.selectedSectionPlane()
	if err != nil {
		return sectionPlane{}, err
	}
	if ok {
		return plane, nil
	}
	midZ := (rbox.Min[2] + rbox.Max[2]) / 2
	return sectionPlane{origin: []float64{0, 0, midZ}, normal: []float64{0, 0, 1}, source: "mid-height"}, nil
}

// selectedSectionPlane resolves the current viewport selection to the plane of the first
// selected planar face, or ok=false when nothing suitable is selected. Faces are matched by
// reference key against model.referenceKeys, and the face normal is read out-of-process via
// body.faceEvaluate at the face's parameter-range midpoint.
func (e *Engine) selectedSectionPlane() (sectionPlane, bool, error) {
	sel, err := e.api.Model().Selection()
	if err != nil {
		return sectionPlane{}, false, err
	}
	if sel.Count == 0 || len(sel.Refs) == 0 {
		return sectionPlane{}, false, nil
	}
	keys, err := e.api.Model().ReferenceKeys()
	if err != nil {
		return sectionPlane{}, false, err
	}
	bodyIndex, key, point, ok := firstSelectedPlanarFace(keys, sel.Refs)
	if !ok {
		return sectionPlane{}, false, nil
	}
	normal, err := e.faceNormal(bodyIndex, key)
	if err != nil {
		return sectionPlane{}, false, err
	}
	return sectionPlane{origin: point, normal: normal, source: "selected face"}, true, nil
}

// firstSelectedPlanarFace returns the body index, reference key, and representative point (cm)
// of the first planar face whose key is in the selection. The selection refs are the same
// reference-key strings model.referenceKeys reports.
func firstSelectedPlanarFace(keys wire.ReferenceKeysResult, refs []string) (bodyIndex int, key string, point []float64, ok bool) {
	selected := make(map[string]bool, len(refs))
	for _, r := range refs {
		if r != "" {
			selected[r] = true
		}
	}
	for bi, b := range keys.Bodies {
		for _, f := range b.Faces {
			if f.Kind == "plane" && len(f.Point) >= 3 && selected[f.Key] {
				return bi, f.Key, []float64{f.Point[0], f.Point[1], f.Point[2]}, true
			}
		}
	}
	return 0, "", nil, false
}

// faceNormal reads a face's unit surface normal via body.faceEvaluate: one call to learn the
// parameter range, a second to evaluate the normal at its midpoint (a point in the face
// interior, so the normal is well-defined even for a trimmed face).
func (e *Engine) faceNormal(bodyIndex int, key string) ([]float64, error) {
	rng, err := e.api.Body().FaceEvaluate(wire.FaceEvaluateArgs{BodyIndex: bodyIndex, FaceKey: key, Mode: wire.FaceEvalNormalAtParam})
	if err != nil {
		return nil, err
	}
	if len(rng.ParamRange) < 4 {
		return nil, fmt.Errorf("face %q evaluation returned a %d-element parameter range, want 4 [uMin,vMin,uMax,vMax]", key, len(rng.ParamRange))
	}
	uMid := (rng.ParamRange[0] + rng.ParamRange[2]) / 2
	vMid := (rng.ParamRange[1] + rng.ParamRange[3]) / 2
	res, err := e.api.Body().FaceEvaluate(wire.FaceEvaluateArgs{BodyIndex: bodyIndex, FaceKey: key, Mode: wire.FaceEvalNormalAtParam, Inputs: []float64{uMid, vMid}})
	if err != nil {
		return nil, err
	}
	if len(res.Normals) < 3 {
		return nil, fmt.Errorf("face %q normal evaluation returned %d normal components, want 3", key, len(res.Normals))
	}
	return []float64{res.Normals[0], res.Normals[1], res.Normals[2]}, nil
}
