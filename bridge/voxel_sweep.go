// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"

	"oblikovati.org/cam/bridge/gcode"
)

// voxelMove is one cutter sweep from from→to (mm) under a given cutter — the unit the material
// simulator advances through, so the toolpath can be replayed one move at a time.
type voxelMove struct {
	cutter   Cutter
	from, to gcode.Vector3
}

// stampCutter clears every grid cell whose centre lies inside the cutter positioned with its tip at
// `tip` (mm). Only cells in the cutter's bounding box are visited, so a stamp is cheap relative to
// the whole grid.
func stampCutter(g *VoxelGrid, c Cutter, tip gcode.Vector3) {
	i0, i1 := cellSpan(tip.X-c.Radius, tip.X+c.Radius, g.Min.X, g.Res, g.Nx)
	j0, j1 := cellSpan(tip.Y-c.Radius, tip.Y+c.Radius, g.Min.Y, g.Res, g.Ny)
	k0, k1 := cellSpan(tip.Z, tip.Z+c.Height, g.Min.Z, g.Res, g.Nz)
	for k := k0; k <= k1; k++ {
		for j := j0; j <= j1; j++ {
			for i := i0; i <= i1; i++ {
				clearIfInside(g, c, tip, i, j, k)
			}
		}
	}
}

// clearIfInside removes cell (i,j,k) when its centre is within the cutter's radius at that height.
func clearIfInside(g *VoxelGrid, c Cutter, tip gcode.Vector3, i, j, k int) {
	p := g.Center(i, j, k)
	r := c.radiusAt(p.Z - tip.Z)
	if r <= 0 {
		return
	}
	dx, dy := p.X-tip.X, p.Y-tip.Y
	if dx*dx+dy*dy <= r*r {
		g.Clear(i, j, k)
	}
}

// cellSpan returns the inclusive cell-index range [lo,hi] covering the world interval [a,b] on one
// axis, clamped to the grid.
func cellSpan(a, b, min, res float64, n int) (int, int) {
	lo := int(math.Floor((a - min) / res))
	hi := int(math.Floor((b - min) / res))
	return clampCell(lo, n), clampCell(hi, n)
}

// clampCell pins a cell index into [0,n-1].
func clampCell(i, n int) int {
	if i < 0 {
		return 0
	}
	if i > n-1 {
		return n - 1
	}
	return i
}

// sweepSegment removes the cutter's swept volume along from→to by stamping at sub-steps no larger
// than one cell, so a fast move never skips cells between stamps.
func sweepSegment(g *VoxelGrid, c Cutter, from, to gcode.Vector3) {
	steps := int(math.Ceil(distance(from, to)/g.Res)) + 1
	for s := 0; s <= steps; s++ {
		t := float64(s) / float64(steps)
		stampCutter(g, c, lerp(from, to, t))
	}
}

// distance is the Euclidean distance between two points (mm).
func distance(a, b gcode.Vector3) float64 {
	dx, dy, dz := b.X-a.X, b.Y-a.Y, b.Z-a.Z
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// lerp linearly interpolates a→b at parameter t∈[0,1].
func lerp(a, b gcode.Vector3, t float64) gcode.Vector3 {
	return gcode.Vector3{X: a.X + (b.X-a.X)*t, Y: a.Y + (b.Y-a.Y)*t, Z: a.Z + (b.Z-a.Z)*t}
}

// flattenCuts turns generated operation results into the ordered move list the material simulator
// replays: one move per motion segment, each carrying its operation's cutter profile. Positions are
// sticky across commands (an axis omitted by a move keeps its previous value).
func flattenCuts(results []OperationResult, fallbackHeight float64) []voxelMove {
	var cuts []voxelMove
	for _, r := range results {
		c := CutterFromTool(r.Controller.Tool, fallbackHeight)
		cuts = append(cuts, opCuts(r.Path, c)...)
	}
	return cuts
}

// opCuts walks one operation's path into cutter moves, tracking the sticky tool position. Canned
// drilling/tapping cycles are first expanded to explicit plunge motion so their holes are carved.
func opCuts(path gcode.Path, c Cutter) []voxelMove {
	var cuts []voxelMove
	var cur gcode.Vector3
	started := false
	for _, cmd := range gcode.ExpandCannedCycles(path).Commands {
		next := applyAxes(cur, cmd)
		if isMotionCommand(cmd.Name) {
			if started {
				cuts = append(cuts, voxelMove{cutter: c, from: cur, to: next})
			}
			started = true
		}
		cur = next
	}
	return cuts
}

// applyAxes returns the position after a command, keeping axes the command omits.
func applyAxes(cur gcode.Vector3, cmd gcode.Command) gcode.Vector3 {
	if x, ok := cmd.Params["X"]; ok {
		cur.X = x
	}
	if y, ok := cmd.Params["Y"]; ok {
		cur.Y = y
	}
	if z, ok := cmd.Params["Z"]; ok {
		cur.Z = z
	}
	return cur
}
