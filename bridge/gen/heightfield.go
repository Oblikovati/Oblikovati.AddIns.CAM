// SPDX-License-Identifier: GPL-2.0-only

package gen

import "math"

// Heightfield is the cutter-location surface sampled on a regular XY grid: Z[i*NY+j] is the
// tool-tip height (mm) the drop-cutter found at grid node (X0+i·Step, Y0+j·Step). A NaN marks a
// node with no data. Waterline finishing slices this field at constant Z to get the level
// contours the cutter rides.
type Heightfield struct {
	X0, Y0, Step float64
	NX, NY       int
	Z            []float64
}

// at returns the height at grid node (i, j).
func (h *Heightfield) at(i, j int) float64 { return h.Z[i*h.NY+j] }

// node returns the XY coordinates of grid node (i, j).
func (h *Heightfield) node(i, j int) [2]float64 {
	return [2]float64{h.X0 + float64(i)*h.Step, h.Y0 + float64(j)*h.Step}
}

// Contour extracts the iso-contour loops of the height field at the given level (mm) via
// marching squares, returning each loop as a list of XY points. Cells touching a NaN node are
// skipped (a hole in the data simply leaves a gap). Ports the classic level-set extraction CAM
// z-level finishing builds its constant-Z passes from.
func (h *Heightfield) Contour(level float64) [][][2]float64 {
	if h.NX < 2 || h.NY < 2 {
		return nil
	}
	// Nudge the level off any exact node value: a corner with z == level would otherwise be
	// ambiguously "inside" and place a crossing exactly on the node, where several cells meet
	// and the chain fragments. An offset far below the grid step keeps the contour in place.
	level += 1e-7
	var segs [][2][2]float64
	for i := 0; i < h.NX-1; i++ {
		for j := 0; j < h.NY-1; j++ {
			segs = appendCellSegments(segs, h, i, j, level)
		}
	}
	return chainSegments(segs)
}

// appendCellSegments adds the iso-line segment(s) crossing one grid cell at the level.
func appendCellSegments(segs [][2][2]float64, h *Heightfield, i, j int, level float64) [][2][2]float64 {
	z := [4]float64{h.at(i, j), h.at(i+1, j), h.at(i+1, j+1), h.at(i, j+1)}
	for _, v := range z {
		if math.IsNaN(v) {
			return segs
		}
	}
	corner := [4][2]float64{h.node(i, j), h.node(i+1, j), h.node(i+1, j+1), h.node(i, j+1)}
	code := 0
	for k, v := range z {
		if v >= level {
			code |= 1 << uint(k)
		}
	}
	// edge k connects corner k and corner (k+1)%4; its crossing point is interpolated.
	edge := func(k int) [2]float64 { return interpEdge(corner[k], corner[(k+1)%4], z[k], z[(k+1)%4], level) }
	for _, e := range marchCases[code] {
		segs = append(segs, [2][2]float64{edge(e[0]), edge(e[1])})
	}
	return segs
}

// marchCases maps a marching-squares corner code (bit k set when corner k ≥ level) to the
// cell edges its contour segments connect (edge k spans corner k→(k+1)%4). The two ambiguous
// saddle cases (5, 10) are resolved one consistent way.
var marchCases = [16][][2]int{
	{},               // 0000
	{{3, 0}},         // 0001
	{{0, 1}},         // 0010
	{{3, 1}},         // 0011
	{{1, 2}},         // 0100
	{{3, 0}, {1, 2}}, // 0101 saddle
	{{0, 2}},         // 0110
	{{3, 2}},         // 0111
	{{2, 3}},         // 1000
	{{2, 0}},         // 1001
	{{2, 3}, {0, 1}}, // 1010 saddle
	{{2, 1}},         // 1011
	{{1, 3}},         // 1100
	{{1, 0}},         // 1101
	{{0, 3}},         // 1110
	{},               // 1111
}

// interpEdge returns the point on edge a→b where the field equals level (linear interpolation).
func interpEdge(a, b [2]float64, za, zb, level float64) [2]float64 {
	d := zb - za
	t := 0.5
	if d != 0 {
		t = (level - za) / d
	}
	return [2]float64{a[0] + t*(b[0]-a[0]), a[1] + t*(b[1]-a[1])}
}
