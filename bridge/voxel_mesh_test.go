// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// TestSingleVoxelMeshIsACube checks one occupied cell yields a six-face cube (4 verts + 2 triangles
// per face, no shared vertices).
func TestSingleVoxelMeshIsACube(t *testing.T) {
	g := NewVoxelGrid(gcode.Vector3{}, gcode.Vector3{X: 1, Y: 1, Z: 1}, 1)
	coords, indices := voxelSurfaceMesh(g)
	if len(coords) != 6*4*3 {
		t.Errorf("coords = %d, want %d", len(coords), 6*4*3)
	}
	if len(indices) != 6*6 {
		t.Errorf("indices = %d, want %d", len(indices), 6*6)
	}
}

// TestInteriorFacesCulled checks the shared face between two adjacent cells is not emitted (each
// cell shows five faces, not six).
func TestInteriorFacesCulled(t *testing.T) {
	g := NewVoxelGrid(gcode.Vector3{}, gcode.Vector3{X: 2, Y: 1, Z: 1}, 1) // two cells side by side
	_, indices := voxelSurfaceMesh(g)
	if len(indices) != 10*6 { // 10 exposed faces, not 12
		t.Errorf("indices = %d, want %d (interior face culled)", len(indices), 10*6)
	}
}

// TestEmptyGridMeshIsEmpty checks a fully carved grid produces no geometry.
func TestEmptyGridMeshIsEmpty(t *testing.T) {
	g := NewVoxelGrid(gcode.Vector3{}, gcode.Vector3{X: 3, Y: 3, Z: 3}, 1)
	for k := 0; k < g.Nz; k++ {
		for j := 0; j < g.Ny; j++ {
			for i := 0; i < g.Nx; i++ {
				g.Clear(i, j, k)
			}
		}
	}
	coords, indices := voxelSurfaceMesh(g)
	if len(coords) != 0 || len(indices) != 0 {
		t.Errorf("empty grid produced %d coords / %d indices", len(coords), len(indices))
	}
}

// TestMeshVertexInWorldSpace checks a cell's face vertices land at the cell's world corners (mm).
func TestMeshVertexInWorldSpace(t *testing.T) {
	g := NewVoxelGrid(gcode.Vector3{X: 5, Y: 5, Z: 5}, gcode.Vector3{X: 7, Y: 6, Z: 6}, 2)
	coords, _ := voxelSurfaceMesh(g) // one cell spanning [5,7]×[5,7]×[5,7]
	if !coordsContain(coords, 5, 5, 5) || !coordsContain(coords, 7, 7, 7) {
		t.Errorf("cube corners (5,5,5)/(7,7,7) missing from mesh")
	}
}

// coordsContain reports whether the xyz triple appears among the coordinate stream.
func coordsContain(coords []float64, x, y, z float64) bool {
	for i := 0; i+2 < len(coords); i += 3 {
		if coords[i] == x && coords[i+1] == y && coords[i+2] == z {
			return true
		}
	}
	return false
}
