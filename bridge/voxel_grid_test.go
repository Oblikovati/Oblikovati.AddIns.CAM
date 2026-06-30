// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// TestNewVoxelGridIsSolid checks a fresh grid is fully occupied and sized by ceil(extent/res).
func TestNewVoxelGridIsSolid(t *testing.T) {
	g := NewVoxelGrid(gcode.Vector3{}, gcode.Vector3{X: 10, Y: 6, Z: 4}, 2)
	if g.Nx != 5 || g.Ny != 3 || g.Nz != 2 {
		t.Fatalf("dims = %d/%d/%d, want 5/3/2", g.Nx, g.Ny, g.Nz)
	}
	if g.Total() != 30 || g.Count() != 30 {
		t.Errorf("total/count = %d/%d, want 30/30 (solid)", g.Total(), g.Count())
	}
}

// TestVoxelClearAndCount checks clearing a voxel drops the occupied count and reads back empty.
func TestVoxelClearAndCount(t *testing.T) {
	g := NewVoxelGrid(gcode.Vector3{}, gcode.Vector3{X: 4, Y: 4, Z: 4}, 1) // 64 voxels
	g.Clear(1, 2, 3)
	if g.Occupied(1, 2, 3) {
		t.Error("voxel still occupied after Clear")
	}
	if g.Count() != 63 {
		t.Errorf("count = %d, want 63", g.Count())
	}
	g.Clear(1, 2, 3) // idempotent
	if g.Count() != 63 {
		t.Errorf("re-clear changed count to %d", g.Count())
	}
}

// TestVoxelCenter checks a voxel's centre is min + (i+0.5)·res.
func TestVoxelCenter(t *testing.T) {
	g := NewVoxelGrid(gcode.Vector3{X: 1, Y: 1, Z: 1}, gcode.Vector3{X: 5, Y: 5, Z: 5}, 2)
	c := g.Center(1, 0, 0)
	if c.X != 4 || c.Y != 2 || c.Z != 2 { // 1 + (1+0.5)*2 = 4 ; 1 + 0.5*2 = 2
		t.Errorf("center = %+v, want {4 2 2}", c)
	}
}

// TestVoxelInBounds checks bounds rejection on every axis.
func TestVoxelInBounds(t *testing.T) {
	g := NewVoxelGrid(gcode.Vector3{}, gcode.Vector3{X: 4, Y: 4, Z: 4}, 1)
	if !g.InBounds(3, 3, 3) || g.InBounds(4, 0, 0) || g.InBounds(0, -1, 0) {
		t.Error("InBounds wrong at edges")
	}
}
