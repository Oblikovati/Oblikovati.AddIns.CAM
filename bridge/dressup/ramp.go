// SPDX-License-Identifier: GPL-2.0-only

package dressup

import (
	"math"

	"oblikovati.org/cam/bridge/gcode"
)

// RampParams configure ramp entry: instead of plunging straight down, the tool descends at an
// angle by zig-zagging back and forth along the next cut direction, which is far gentler on the
// cutter. Length is the run of each back-and-forth (mm); Angle is the descent angle (radians).
type RampParams struct {
	Length float64
	Angle  float64
}

// maxRampPasses caps the zig-zag passes so a very shallow angle can't explode the path.
const maxRampPasses = 100

// ApplyRamp replaces each straight plunge (a G1 that lowers only Z) with a ramped descent along
// the following cut's direction. A plunge with no subsequent cutting move, or zero params, is
// left as a straight plunge.
func ApplyRamp(path gcode.Path, p RampParams) gcode.Path {
	if p.Length <= 0 || p.Angle <= 0 {
		return path
	}
	out := gcode.Path{Commands: make([]gcode.Command, 0, len(path.Commands))}
	var px, py, pz float64
	posKnown := false
	for i, c := range path.Commands {
		if posKnown && isPlunge(c, px, py, pz) {
			if dx, dy, ok := nextCutDir(path.Commands[i+1:], px, py); ok {
				toZ := c.Params["Z"]
				out.Commands = append(out.Commands, rampMoves(px, py, pz, toZ, dx, dy, feedOf(c), p)...)
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

// isPlunge reports whether a command lowers only Z (a straight plunge) from the current height.
func isPlunge(c gcode.Command, px, py, pz float64) bool {
	if c.Name != "G1" {
		return false
	}
	z, hasZ := c.Params["Z"]
	_, hasX := c.Params["X"]
	_, hasY := c.Params["Y"]
	return hasZ && !hasX && !hasY && z < pz
}

// nextCutDir returns the unit XY direction from (px,py) toward the first following move that has
// an X or Y, or ok=false when there is none / it is coincident.
func nextCutDir(rest []gcode.Command, px, py float64) (dx, dy float64, ok bool) {
	for _, c := range rest {
		nx, ny, _, hasXY := endpoint(c, px, py, 0)
		if !hasXY {
			continue
		}
		ex, ey := nx-px, ny-py
		d := math.Hypot(ex, ey)
		if d < 1e-9 {
			return 0, 0, false
		}
		return ex / d, ey / d, true
	}
	return 0, 0, false
}

// rampMoves builds the zig-zag descent from fromZ to toZ along (dx,dy), ending back at (px,py).
func rampMoves(px, py, fromZ, toZ, dx, dy, feed float64, p RampParams) []gcode.Command {
	depth := fromZ - toZ
	travel := depth / math.Tan(p.Angle) // horizontal distance to lose the depth at the angle
	n := int(math.Ceil(travel / p.Length))
	if n < 1 {
		n = 1
	}
	if n > maxRampPasses {
		n = maxRampPasses
	}
	dz := depth / float64(n)
	cmds := make([]gcode.Command, 0, n+1)
	z := fromZ
	away := false // false = currently at the plunge point, true = at the far end of the ramp
	for i := 0; i < n; i++ {
		if z -= dz; z < toZ {
			z = toZ
		}
		if away {
			cmds = append(cmds, rampMove(px, py, z, feed))
		} else {
			cmds = append(cmds, rampMove(px+dx*p.Length, py+dy*p.Length, z, feed))
		}
		away = !away
	}
	if away { // ended at the far end → return to the plunge point at depth
		cmds = append(cmds, rampMove(px, py, toZ, feed))
	}
	return cmds
}

// rampMove is one feed move of a ramp (X/Y/Z, optional feed).
func rampMove(x, y, z, feed float64) gcode.Command {
	params := map[string]float64{"X": x, "Y": y, "Z": z}
	if feed > 0 {
		params["F"] = feed
	}
	return gcode.NewCommand("G1", params)
}

// feedOf returns a command's feed, or 0.
func feedOf(c gcode.Command) float64 { return c.Params["F"] }
