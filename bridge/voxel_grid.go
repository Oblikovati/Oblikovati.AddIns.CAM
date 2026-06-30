// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"math/bits"

	"oblikovati.org/cam/bridge/gcode"
)

// VoxelGrid is a uniform occupancy grid over an axis-aligned stock box: one bit per cubic cell
// (Res mm on a side), 1 = material present. The material-removal simulator starts it solid and
// clears cells the cutter sweeps through. Cell (i,j,k) spans [Min+ijk·Res, Min+(ijk+1)·Res); its
// centre is the point tested against the cutter. Occupancy is a packed bitset for memory: a
// 96×96×96 grid is ~110 KB, so even a few-million-cell stock stays small.
type VoxelGrid struct {
	Min        gcode.Vector3 // mm, the (0,0,0) cell's low corner
	Res        float64       // mm per cell edge
	Nx, Ny, Nz int           // cell counts per axis
	occ        []uint64      // occupancy bitset, idx = (k*Ny+j)*Nx + i
}

// NewVoxelGrid returns a fully solid grid covering [min,max] (mm) at the given cell size. Each axis
// gets ceil(extent/res) cells (at least one), so the grid always encloses the box.
func NewVoxelGrid(min, max gcode.Vector3, res float64) *VoxelGrid {
	nx := axisCells(max.X-min.X, res)
	ny := axisCells(max.Y-min.Y, res)
	nz := axisCells(max.Z-min.Z, res)
	g := &VoxelGrid{Min: min, Res: res, Nx: nx, Ny: ny, Nz: nz}
	g.occ = make([]uint64, (nx*ny*nz+63)/64)
	g.fillSolid()
	return g
}

// axisCells is the cell count spanning an extent at a cell size, at least one.
func axisCells(extent, res float64) int {
	n := int(math.Ceil(extent / res))
	if n < 1 {
		return 1
	}
	return n
}

// fillSolid marks every in-range cell occupied, leaving the bitset's tail padding clear so Count
// stays exact.
func (g *VoxelGrid) fillSolid() {
	for w := range g.occ {
		g.occ[w] = ^uint64(0)
	}
	for idx := g.Total(); idx < len(g.occ)*64; idx++ {
		g.occ[idx>>6] &^= 1 << (uint(idx) & 63)
	}
}

// Total is the number of cells in the grid.
func (g *VoxelGrid) Total() int { return g.Nx * g.Ny * g.Nz }

// index packs a cell coordinate into its bitset position.
func (g *VoxelGrid) index(i, j, k int) int { return (k*g.Ny+j)*g.Nx + i }

// InBounds reports whether (i,j,k) is a cell of the grid.
func (g *VoxelGrid) InBounds(i, j, k int) bool {
	return i >= 0 && i < g.Nx && j >= 0 && j < g.Ny && k >= 0 && k < g.Nz
}

// Occupied reports whether cell (i,j,k) holds material (false when out of bounds).
func (g *VoxelGrid) Occupied(i, j, k int) bool {
	if !g.InBounds(i, j, k) {
		return false
	}
	idx := g.index(i, j, k)
	return g.occ[idx>>6]&(1<<(uint(idx)&63)) != 0
}

// Clear removes the material in cell (i,j,k); out-of-bounds and already-empty cells are no-ops.
func (g *VoxelGrid) Clear(i, j, k int) {
	if !g.InBounds(i, j, k) {
		return
	}
	idx := g.index(i, j, k)
	g.occ[idx>>6] &^= 1 << (uint(idx) & 63)
}

// Count is the number of occupied cells (population count over the bitset).
func (g *VoxelGrid) Count() int {
	n := 0
	for _, w := range g.occ {
		n += bits.OnesCount64(w)
	}
	return n
}

// cellAt returns the cell containing world point p (mm) and whether it lies in the grid.
func (g *VoxelGrid) cellAt(p gcode.Vector3) (i, j, k int, ok bool) {
	i = int(math.Floor((p.X - g.Min.X) / g.Res))
	j = int(math.Floor((p.Y - g.Min.Y) / g.Res))
	k = int(math.Floor((p.Z - g.Min.Z) / g.Res))
	return i, j, k, g.InBounds(i, j, k)
}

// Center is the centre point (mm) of cell (i,j,k).
func (g *VoxelGrid) Center(i, j, k int) gcode.Vector3 {
	return gcode.Vector3{
		X: g.Min.X + (float64(i)+0.5)*g.Res,
		Y: g.Min.Y + (float64(j)+0.5)*g.Res,
		Z: g.Min.Z + (float64(k)+0.5)*g.Res,
	}
}
