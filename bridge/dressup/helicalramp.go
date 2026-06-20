// SPDX-License-Identifier: GPL-2.0-only

package dressup

import (
	"math"

	"oblikovati.org/cam/bridge/gcode"
)

// HelicalRampParams configure a helical ramp entry: instead of plunging straight down, the tool
// spirals down a circle tangent to the first cut, boring an entry helix. This is the gentle entry
// for closed pockets where there is no room for a back-and-forth linear ramp. Radius is the helix
// radius (mm); Pitch is the depth descended per full turn (mm) — a smaller pitch is gentler.
type HelicalRampParams struct {
	Radius float64
	Pitch  float64
}

// helixSegmentsPerTurn samples each full turn of the entry helix into this many feed segments,
// enough that the plotted/cut circle reads smoothly (the toolpath model is polyline, like the
// offset rings; the post can re-fit arcs if it wants).
const helixSegmentsPerTurn = 24

// maxHelixTurns caps the descent so a tiny pitch cannot explode the path.
const maxHelixTurns = 200

// ApplyHelicalRamp replaces each straight plunge (a G1 lowering only Z) with a helical descent on
// a circle tangent to the following cut at the plunge point, ending back at the plunge point at
// full depth so the subsequent cutting moves continue unchanged. A plunge with no following cut,
// or zero params, is left as a straight plunge.
func ApplyHelicalRamp(path gcode.Path, p HelicalRampParams) gcode.Path {
	if p.Radius <= 0 || p.Pitch <= 0 {
		return path
	}
	out := gcode.Path{Commands: make([]gcode.Command, 0, len(path.Commands))}
	var px, py, pz float64
	posKnown := false
	for i, c := range path.Commands {
		if posKnown && isPlunge(c, px, py, pz) {
			if dx, dy, ok := nextCutDir(path.Commands[i+1:], px, py); ok {
				toZ := c.Params["Z"]
				out.Commands = append(out.Commands, helixMoves(px, py, pz, toZ, dx, dy, feedOf(c), p)...)
				pz = toZ
				continue
			}
		}
		out.Commands = append(out.Commands, c)
		nx, ny, nz, hasXY := endpoint(c, px, py, pz)
		px, py, pz = nx, ny, nz
		posKnown = posKnown || hasXY
	}
	return out
}

// helixMoves builds the helical descent from fromZ to toZ on a circle of radius p.Radius whose
// centre sits one radius to the left of the cut direction (dx,dy), so the plunge point lies on the
// circle and the helix is tangent to the first cut there. It spirals (CCW) for as many turns as
// the depth needs at the pitch, descending linearly, and finishes back at the plunge point.
func helixMoves(px, py, fromZ, toZ, dx, dy, feed float64, p HelicalRampParams) []gcode.Command {
	depth := fromZ - toZ
	turns := int(math.Ceil(depth / p.Pitch))
	if turns < 1 {
		turns = 1
	}
	if turns > maxHelixTurns {
		turns = maxHelixTurns
	}
	// Centre one radius along the cut's left normal; the plunge point is then on the circle.
	cx, cy := px-dy*p.Radius, py+dx*p.Radius
	start := math.Atan2(py-cy, px-cx)
	steps := turns * helixSegmentsPerTurn
	total := 2 * math.Pi * float64(turns)

	cmds := make([]gcode.Command, 0, steps)
	for k := 1; k <= steps; k++ {
		frac := float64(k) / float64(steps)
		ang := start + total*frac // CCW, tangent to (dx,dy) at the plunge point
		z := fromZ - depth*frac
		cmds = append(cmds, rampMove(cx+p.Radius*math.Cos(ang), cy+p.Radius*math.Sin(ang), z, feed))
	}
	// The last sample lands back on the plunge point (integer turns) at toZ; pin it exactly.
	cmds = append(cmds, rampMove(px, py, toZ, feed))
	return cmds
}
