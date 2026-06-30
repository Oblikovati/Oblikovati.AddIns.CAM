// SPDX-License-Identifier: GPL-2.0-only

package bridge

// Surface extraction for the material-removal simulator: emit the boundary between solid and empty
// cells as quads (the classic blocky "exposed-face" mesh). A face is drawn only where an occupied
// cell meets an empty cell or the grid edge, so interior faces are culled. The result is honest
// about voxel resolution and free of the marching-cubes ambiguities — good enough for a live
// machining preview. Vertices are not shared between faces (each quad carries its own four).

// voxFace is one of a cube's six faces: the neighbour direction that must be empty for the face to
// show, and its four corners (each 0/1 per axis) wound counter-clockwise as seen from outside.
type voxFace struct {
	di, dj, dk int
	corners    [4][3]int
}

// voxFaces lists the six cube faces with outward winding (so triangle normals point out of solid).
var voxFaces = []voxFace{
	{1, 0, 0, [4][3]int{{1, 0, 0}, {1, 1, 0}, {1, 1, 1}, {1, 0, 1}}},  // +X
	{-1, 0, 0, [4][3]int{{0, 0, 0}, {0, 0, 1}, {0, 1, 1}, {0, 1, 0}}}, // -X
	{0, 1, 0, [4][3]int{{0, 1, 0}, {0, 1, 1}, {1, 1, 1}, {1, 1, 0}}},  // +Y
	{0, -1, 0, [4][3]int{{0, 0, 0}, {1, 0, 0}, {1, 0, 1}, {0, 0, 1}}}, // -Y
	{0, 0, 1, [4][3]int{{0, 0, 1}, {1, 0, 1}, {1, 1, 1}, {0, 1, 1}}},  // +Z
	{0, 0, -1, [4][3]int{{0, 0, 0}, {0, 1, 0}, {1, 1, 0}, {1, 0, 0}}}, // -Z
}

// voxelSurfaceMesh returns the indexed triangle mesh (coords as xyz triples in mm, indices as
// triangle vertex indices) of the grid's solid/empty boundary.
func voxelSurfaceMesh(g *VoxelGrid) ([]float64, []int) {
	var coords []float64
	var indices []int
	for k := 0; k < g.Nz; k++ {
		for j := 0; j < g.Ny; j++ {
			for i := 0; i < g.Nx; i++ {
				if g.Occupied(i, j, k) {
					coords, indices = emitCellFaces(g, i, j, k, coords, indices)
				}
			}
		}
	}
	return coords, indices
}

// emitCellFaces appends the exposed faces of cell (i,j,k) — those whose neighbour is empty — to the
// growing mesh.
func emitCellFaces(g *VoxelGrid, i, j, k int, coords []float64, indices []int) ([]float64, []int) {
	for _, f := range voxFaces {
		if g.Occupied(i+f.di, j+f.dj, k+f.dk) {
			continue
		}
		base := len(coords) / 3
		coords = appendFaceCorners(g, i, j, k, f, coords)
		indices = append(indices, base, base+1, base+2, base, base+2, base+3)
	}
	return coords, indices
}

// appendFaceCorners appends a face's four world-space corner vertices (mm) to the coordinate stream.
func appendFaceCorners(g *VoxelGrid, i, j, k int, f voxFace, coords []float64) []float64 {
	for _, c := range f.corners {
		coords = append(coords,
			g.Min.X+float64(i+c[0])*g.Res,
			g.Min.Y+float64(j+c[1])*g.Res,
			g.Min.Z+float64(k+c[2])*g.Res)
	}
	return coords
}
