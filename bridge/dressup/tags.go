// SPDX-License-Identifier: GPL-2.0-only

// Package dressup holds toolpath dressups: transforms applied to a generated path to add
// manufacturing features the raw geometry does not — holding tabs, dogbone corner relief, …
// A dressup takes a gcode.Path and returns a modified one, leaf of the CAM add-in (it depends
// only on the toolpath model).
package dressup

import (
	"math"
	"sort"

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

// ApplyTags lifts the tool over evenly spaced holding tabs on a profile path. Each cutting
// segment is split at the tab boundaries (centre ± Width/2) so the lifted bridge is exactly
// Width long regardless of how the contour is discretised — a tab on a long straight edge is a
// short bridge, not the whole edge — with the tool ramping up to cut depth + Height at the tab
// and back down after it. Cutting moves outside any tab are pinned to the cut depth. A zero
// count or width returns the path unchanged.
func ApplyTags(path gcode.Path, p TagParams) gcode.Path {
	if p.Count <= 0 || p.Width <= 0 {
		return path
	}
	cuts := cuttingMoves(path)
	if len(cuts) == 0 || cuts[len(cuts)-1].end <= 0 {
		return path
	}
	spans := tabSpans(cuts[len(cuts)-1].end, p.Count, p.Width/2)
	byIndex := make(map[int]cutMove, len(cuts))
	for _, m := range cuts {
		byIndex[m.index] = m
	}

	out := make([]gcode.Command, 0, len(path.Commands)+len(spans)*2)
	for i, c := range path.Commands {
		if m, ok := byIndex[i]; ok {
			out = append(out, splitOverTabs(c, m, spans, p.Height)...)
		} else {
			out = append(out, cloneCommand(c))
		}
	}
	return gcode.Path{Commands: out}
}

// splitOverTabs turns one cutting move into the sub-moves that walk it: the segment is cut at
// every tab boundary it crosses, and each sub-move ends with its Z set to the cut depth, or the
// cut depth + height when its midpoint falls inside a tab. A move clear of every tab becomes a
// single sub-move at the cut depth.
func splitOverTabs(c gcode.Command, m cutMove, spans []tabSpan, height float64) []gcode.Command {
	stops := []float64{m.start}
	for _, s := range spans {
		stops = appendIfInside(stops, s.lo, m.start, m.end)
		stops = appendIfInside(stops, s.hi, m.start, m.end)
	}
	stops = append(stops, m.end)
	sort.Float64s(stops)

	length := m.end - m.start
	moves := make([]gcode.Command, 0, len(stops)-1)
	for k := 1; k < len(stops); k++ {
		t := (stops[k] - m.start) / length
		z := m.cutZ
		if insideTab((stops[k-1]+stops[k])/2, spans) {
			z += height
		}
		params := map[string]float64{"X": m.sx + (m.ex-m.sx)*t, "Y": m.sy + (m.ey-m.sy)*t, "Z": z}
		if f, ok := c.Params["F"]; ok {
			params["F"] = f
		}
		moves = append(moves, gcode.NewCommand("G1", params))
	}
	return moves
}

// appendIfInside adds a tab-boundary arc-length to the split stops when it falls strictly inside
// the segment (start, end).
func appendIfInside(stops []float64, at, start, end float64) []float64 {
	if at > start && at < end {
		return append(stops, at)
	}
	return stops
}

// cutMove records a cutting move: its index in the path, the cutting-arc-length span its segment
// covers ([start, end]), the segment's XY endpoints, and the cut depth in effect.
type cutMove struct {
	index          int
	start, end     float64
	sx, sy, ex, ey float64
	cutZ           float64
}

// cuttingMoves walks the path tracking the current Z (any move carrying a Z), the XY position
// (any move carrying X/Y), and the cumulative cutting arc length, returning one record per
// cutting (G1 with X or Y) move. A pure-Z plunge updates the depth without resetting the position.
func cuttingMoves(path gcode.Path) []cutMove {
	var moves []cutMove
	var curZ, px, py, dist float64
	posKnown := false
	for i, c := range path.Commands {
		if z, ok := c.Params["Z"]; ok {
			curZ = z
		}
		nx, ny := px, py
		if x, ok := c.Params["X"]; ok {
			nx = x
		}
		y, hasY := c.Params["Y"]
		_, hasX := c.Params["X"]
		if hasY {
			ny = y
		}
		if c.Name == "G1" && (hasX || hasY) && posKnown {
			start := dist
			dist += math.Hypot(nx-px, ny-py)
			moves = append(moves, cutMove{index: i, start: start, end: dist, sx: px, sy: py, ex: nx, ey: ny, cutZ: curZ})
		}
		if hasX || hasY {
			px, py, posKnown = nx, ny, true
		}
	}
	return moves
}

// tabSpan is a tab's arc-length interval [lo, hi] along the contour.
type tabSpan struct{ lo, hi float64 }

// tabSpans returns Count tab spans spaced evenly along a contour of the given total length, each
// centred in its equal share and halfWidth to either side.
func tabSpans(total float64, count int, halfWidth float64) []tabSpan {
	share := total / float64(count)
	spans := make([]tabSpan, count)
	for i := range spans {
		c := share * (float64(i) + 0.5)
		spans[i] = tabSpan{lo: c - halfWidth, hi: c + halfWidth}
	}
	return spans
}

// insideTab reports whether an arc-length position falls within any tab span.
func insideTab(at float64, spans []tabSpan) bool {
	for _, s := range spans {
		if at >= s.lo && at <= s.hi {
			return true
		}
	}
	return false
}

// cloneCommand deep-copies a command so the dressup does not mutate the caller's path.
func cloneCommand(c gcode.Command) gcode.Command {
	params := make(map[string]float64, len(c.Params))
	for k, v := range c.Params {
		params[k] = v
	}
	return gcode.Command{Name: c.Name, Params: params}
}

// clonePath deep-copies a path so a dressup does not mutate the caller's commands.
func clonePath(path gcode.Path) gcode.Path {
	out := gcode.Path{Commands: make([]gcode.Command, len(path.Commands))}
	for i, c := range path.Commands {
		out.Commands[i] = cloneCommand(c)
	}
	return out
}
