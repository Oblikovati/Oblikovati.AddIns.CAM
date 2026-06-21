// SPDX-License-Identifier: GPL-2.0-only

// Package dressup holds toolpath dressups: transforms applied to a generated path to add
// manufacturing features the raw geometry does not — holding tabs, dogbone corner relief, …
// A dressup takes a gcode.Path and returns a modified one, leaf of the CAM add-in (it depends
// only on the toolpath model).
package dressup

import (
	"math"

	"oblikovati.org/cam/bridge/gcode"
)

// TagParams configure holding tabs: Count tabs spaced evenly by
// cutting arc-length, each Width long, lifting the tool Height above the cut so a bridge of
// material is left holding the part to the stock.
type TagParams struct {
	Count  int     // number of tabs around the contour
	Width  float64 // tab length along the path (mm)
	Height float64 // tab height above the cut depth (mm)
}

// ApplyTags lifts the tool over evenly spaced holding tabs on a profile path: cutting moves
// (G1 with X/Y) whose position along the contour falls within a tab span are raised to the
// cut depth + Height, leaving uncut bridges. Non-tab cutting moves are pinned to the cut
// depth so the tool descends back after each tab. A zero count or width returns the path
// unchanged.
func ApplyTags(path gcode.Path, p TagParams) gcode.Path {
	if p.Count <= 0 || p.Width <= 0 {
		return path
	}
	cuts := cuttingMoves(path)
	if len(cuts) == 0 {
		return path
	}
	total := cuts[len(cuts)-1].end
	if total <= 0 {
		return path
	}
	centers := tabCenters(total, p.Count)

	out := clonePath(path)
	for _, m := range cuts {
		z := m.cutZ
		if withinTab(m.start, m.end, centers, p.Width/2) {
			z += p.Height
		}
		out.Commands[m.index].Params["Z"] = z
	}
	return out
}

// cutMove records a cutting move: its index in the path, the cutting-arc-length span its
// segment covers ([start, end]), and the cut depth in effect.
type cutMove struct {
	index      int
	start, end float64
	cutZ       float64
}

// cuttingMoves walks the path tracking the current Z (any move carrying a Z), the XY
// position (any move carrying X/Y), and the cumulative cutting arc length, returning one
// record per cutting (G1 with X or Y) move. A pure-Z plunge updates the depth without
// resetting the position.
func cuttingMoves(path gcode.Path) []cutMove {
	var moves []cutMove
	var curZ, px, py, dist float64
	posKnown := false
	for i, c := range path.Commands {
		if z, ok := c.Params["Z"]; ok {
			curZ = z
		}
		nx, ny := px, py
		x, hasX := c.Params["X"]
		y, hasY := c.Params["Y"]
		if hasX {
			nx = x
		}
		if hasY {
			ny = y
		}
		if c.Name == "G1" && (hasX || hasY) && posKnown {
			start := dist
			dist += math.Hypot(nx-px, ny-py)
			moves = append(moves, cutMove{index: i, start: start, end: dist, cutZ: curZ})
		}
		if hasX || hasY {
			px, py, posKnown = nx, ny, true
		}
	}
	return moves
}

// tabCenters returns Count tab centre positions spaced evenly along a contour of the given
// total length (each at the middle of its equal share).
func tabCenters(total float64, count int) []float64 {
	span := total / float64(count)
	centers := make([]float64, count)
	for i := range centers {
		centers[i] = span * (float64(i) + 0.5)
	}
	return centers
}

// withinTab reports whether a move's arc-length span [start, end] overlaps any tab span
// (centre ± halfWidth) — so a tab landing mid-segment still lifts that move.
func withinTab(start, end float64, centers []float64, halfWidth float64) bool {
	for _, c := range centers {
		if c-halfWidth <= end && c+halfWidth >= start {
			return true
		}
	}
	return false
}

// clonePath deep-copies a path so the dressup does not mutate the caller's commands.
func clonePath(path gcode.Path) gcode.Path {
	out := gcode.Path{Commands: make([]gcode.Command, len(path.Commands))}
	for i, c := range path.Commands {
		params := make(map[string]float64, len(c.Params))
		for k, v := range c.Params {
			params[k] = v
		}
		out.Commands[i] = gcode.Command{Name: c.Name, Params: params}
	}
	return out
}
