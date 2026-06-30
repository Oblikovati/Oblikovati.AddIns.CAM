// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// TestMaterialMeshCarvesAndMeshes checks the headless pipeline entry runs a job end-to-end into a
// non-empty carved surface mesh at a sane voxel size.
func TestMaterialMeshCarvesAndMeshes(t *testing.T) {
	coords, indices, res, err := MaterialMesh(materialJob())
	if err != nil {
		t.Fatalf("MaterialMesh: %v", err)
	}
	if len(coords) == 0 || len(indices) == 0 {
		t.Fatalf("empty mesh: %d coords / %d indices", len(coords), len(indices))
	}
	if len(indices)%3 != 0 {
		t.Errorf("indices = %d, not a whole number of triangles", len(indices))
	}
	if res <= 0 {
		t.Errorf("voxel size = %v, want > 0", res)
	}
}

// TestMaterialMeshErrorsWithoutCuts checks a job with no cutting moves is reported, not silently
// meshed as a solid block.
func TestMaterialMeshErrorsWithoutCuts(t *testing.T) {
	job := &Job{Stock: Stock{Max: gcode.Vector3{X: 10, Y: 10, Z: 10}}} // no operations → no cuts
	if _, _, _, err := MaterialMesh(job); err == nil {
		t.Fatal("expected an error for a job with no cuts")
	}
}
