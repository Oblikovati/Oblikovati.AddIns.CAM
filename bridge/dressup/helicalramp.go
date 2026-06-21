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
	return ApplyHelicalRampBounded(path, p, nil)
}

// ApplyHelicalRampBounded is ApplyHelicalRamp with a wall-clearance guard: roomAt(x,y) reports the
// distance from a point to the nearest wall, and each plunge's helix radius is shrunk so the entry
// circle's disk stays inside that room — the helix never swings into a wall it has no business
// touching (the gouge a fixed radius risks in a tight pocket or neck). A nil roomAt disables the
// guard, leaving the radius exactly as configured.
func ApplyHelicalRampBounded(path gcode.Path, p HelicalRampParams, roomAt func(x, y float64) float64) gcode.Path {
	if p.Radius <= 0 || p.Pitch <= 0 {
		return path
	}
	out := gcode.Path{Commands: make([]gcode.Command, 0, len(path.Commands))}
	var px, py, pz float64
	posKnown := false
	for i, c := range path.Commands {
		if posKnown && isPlunge(c, px, py, pz) {
			if dx, dy, _, ok := nextCutDir(path.Commands[i+1:], px, py); ok {
				toZ := c.Params["Z"]
				pass := p
				pass.Radius = clampHelixRadius(px, py, dx, dy, p.Radius, roomAt)
				out.Commands = append(out.Commands, helixMoves(px, py, pz, toZ, dx, dy, feedOf(c), pass)...)
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

// clampHelixRadius returns the largest radius up to the requested one whose entry-helix disk (a
// circle of that radius centred one radius along the cut's left normal, per helixMoves) stays
// within the room reported by roomAt — i.e. roomAt(centre) >= radius. roomAt nil leaves the radius
// unchanged. The centre moves further from the plunge point as the radius grows, so the fit
// shrinks monotonically and a bisection on [0, requested] converges on the largest safe radius.
func clampHelixRadius(px, py, dx, dy, requested float64, roomAt func(x, y float64) float64) float64 {
	if roomAt == nil {
		return requested
	}
	fits := func(r float64) bool { return roomAt(px-dy*r, py+dx*r) >= r }
	if fits(requested) {
		return requested
	}
	lo, hi := 0.0, requested
	for i := 0; i < 32; i++ { // bisect to the largest radius that still fits
		mid := (lo + hi) / 2
		if fits(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
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
