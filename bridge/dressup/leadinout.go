// SPDX-License-Identifier: GPL-2.0-only

package dressup

import (
	"math"

	"oblikovati.org/cam/bridge/gcode"
)

// LeadInOutParams configure lead-in/lead-out: instead of plunging straight onto the contour
// and starting the cut at full engagement, the tool plunges a tangential arc away from the
// contour, then sweeps in along a quarter-circle so it eases into (and out of) the material.
// Radius is the lead arc radius (mm). Side selects which side of the cut direction the arc
// curves toward: SideLeft or SideRight. A small radius makes a tight lead; the side is chosen
// so the arc stays clear of the finished wall.
type LeadInOutParams struct {
	Radius float64
	Side   string
}

// The lead arc side reuses the package's SideLeft / SideRight constants (see dogbone.go).

// ApplyLeadInOut wraps each cut sequence (a positioning rapid → straight plunge → run of feed
// moves → retract) with a tangential quarter-arc lead-in and lead-out. The plunge is relocated
// to the lead-in arc's start so the tool descends clear of the contour, sweeps in tangentially,
// cuts the sequence, then sweeps out tangentially before retracting. Zero/negative radius, or a
// sequence without a following cut, is left untouched.
func ApplyLeadInOut(path gcode.Path, p LeadInOutParams) gcode.Path {
	if p.Radius <= 0 {
		return path
	}
	side := leadSign(p.Side)
	out := gcode.Path{Commands: make([]gcode.Command, 0, len(path.Commands)+2*sequenceEstimate(path))}
	var px, py, pz float64
	rapidIdx := -1 // index in out of the most recent XY-positioning rapid
	posKnown := false
	for i := 0; i < len(path.Commands); i++ {
		c := path.Commands[i]
		if isPositioningRapid(c) {
			rapidIdx = len(out.Commands)
		}
		if posKnown && rapidIdx >= 0 && isPlunge(c, px, py, pz) {
			if h := leadInPlunge(&out, path.Commands, i, px, py, rapidIdx, side, p.Radius); h != nil {
				i, px, py, pz = h.i, h.x, h.y, h.z
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

// leadResult carries the cursor + position after a lead-wrapped cut sequence is emitted; a nil
// result means the plunge was not a lead sequence and should be emitted straight.
type leadResult struct {
	i       int
	x, y, z float64
}

// leadInPlunge handles a plunge that begins a cut sequence: it relocates the positioning rapid
// and plunge to the lead-in arc start, sweeps in to the contour, copies the sequence's cut
// moves, then sweeps out before the retract. Returns ok=false when the plunge has no following
// cut (so the caller leaves it as a straight plunge). px,py is the contour start the plunge
// would have entered at.
func leadInPlunge(out *gcode.Path, cmds []gcode.Command, plungeIdx int, px, py float64, rapidIdx int, side, radius float64) *leadResult {
	depth := cmds[plungeIdx].Params["Z"]
	dx, dy, ok := nextCutDir(cmds[plungeIdx+1:], px, py)
	if !ok {
		return nil
	}
	startX, startY := leadArcStart(px, py, dx, dy, side, radius)
	relocateRapid(out, rapidIdx, startX, startY)
	out.Commands = append(out.Commands, cmds[plungeIdx]) // plunge, now at the lead-in start

	feed := feedOf(cmds[plungeIdx+1])
	out.Commands = append(out.Commands, leadArc(startX, startY, px, py, dx, dy, depth, side, radius, feed, true))

	// copy the cut moves of this sequence, tracking the running end + final direction
	cx, cy, cz := px, py, depth
	j := plungeIdx + 1
	var lastDX, lastDY float64
	haveLast := false
	for ; j < len(cmds); j++ {
		if isRetract(cmds[j], cz) {
			break
		}
		nx, ny, nz, hasXY := endpoint(cmds[j], cx, cy, cz)
		if hasXY {
			if ex, ey := nx-cx, ny-cy; math.Hypot(ex, ey) > 1e-9 {
				lastDX, lastDY, haveLast = unit(ex, ey)
			}
		}
		out.Commands = append(out.Commands, cmds[j])
		cx, cy, cz = nx, ny, nz
	}
	if haveLast {
		outX, outY := leadArcStart(cx, cy, lastDX, lastDY, side, radius) // exit arc mirrors the entry
		out.Commands = append(out.Commands, leadArc(cx, cy, outX, outY, lastDX, lastDY, cz, side, radius, feed, false))
		cx, cy = outX, outY
	}
	return &leadResult{i: j - 1, x: cx, y: cy, z: cz}
}

// isPositioningRapid reports whether a command is a rapid (G0) that moves in XY — the move that
// places the tool over the contour start before a plunge.
func isPositioningRapid(c gcode.Command) bool {
	if c.Name != "G0" {
		return false
	}
	_, hasX := c.Params["X"]
	_, hasY := c.Params["Y"]
	return hasX || hasY
}

// isRetract reports whether a command rapids up in Z (the move that ends a cut sequence).
func isRetract(c gcode.Command, cz float64) bool {
	if c.Name != "G0" {
		return false
	}
	z, hasZ := c.Params["Z"]
	return hasZ && z > cz
}

// relocateRapid rewrites the positioning rapid's X/Y to the lead-in arc start so the tool
// descends clear of the contour.
func relocateRapid(out *gcode.Path, idx int, x, y float64) {
	c := out.Commands[idx]
	params := make(map[string]float64, len(c.Params))
	for k, v := range c.Params {
		params[k] = v
	}
	params["X"], params["Y"] = x, y
	out.Commands[idx] = gcode.NewCommand(c.Name, params)
}

// leadArcStart returns the point a quarter lead arc starts from to reach (x,y) tangentially in
// direction (dx,dy): the arc centre sits perpendicular (to the chosen side) at the radius, and
// the start is a further quarter turn back. The same geometry, run forward, places the lead-out
// exit point.
func leadArcStart(x, y, dx, dy, side, radius float64) (sx, sy float64) {
	nx, ny := side*-dy, side*dx // unit normal to the travel direction on the chosen side
	cx, cy := x+nx*radius, y+ny*radius
	return cx - dx*radius, cy - dy*radius
}

// leadArc builds the quarter-circle G2/G3 lead move. For a lead-in (entering=true) it runs from
// the arc start to the contour point (bx,by); for a lead-out it runs from the contour point to
// the exit. The centre offset (I,J) is relative to the move's begin, per G-code arc convention.
func leadArc(fromX, fromY, toX, toY, dx, dy, z, side, radius, feed float64, entering bool) gcode.Command {
	bx, by := fromX, fromY
	nx, ny := side*-dy, side*dx
	var cx, cy float64
	if entering {
		cx, cy = toX+nx*radius, toY+ny*radius // centre defined from the contour (tangent) point
	} else {
		cx, cy = fromX+nx*radius, fromY+ny*radius
	}
	name := "G2" // CW
	if side > 0 {
		name = "G3" // CCW lead on the left side
	}
	params := map[string]float64{"X": toX, "Y": toY, "Z": z, "I": cx - bx, "J": cy - by}
	if feed > 0 {
		params["F"] = feed
	}
	return gcode.NewCommand(name, params)
}

// leadSign maps a side name to the perpendicular sign (+1 left, -1 right); unknown defaults to
// left.
func leadSign(side string) float64 {
	if side == SideRight {
		return -1
	}
	return 1
}

// unit normalises (x,y), reporting ok=false for a near-zero vector.
func unit(x, y float64) (ux, uy float64, ok bool) {
	d := math.Hypot(x, y)
	if d < 1e-9 {
		return 0, 0, false
	}
	return x / d, y / d, true
}

// sequenceEstimate counts plunges as a cheap upper bound on lead sequences, to size the output
// slice.
func sequenceEstimate(path gcode.Path) int {
	n := 0
	for _, c := range path.Commands {
		if c.Name == "G1" {
			if _, hasZ := c.Params["Z"]; hasZ {
				if _, hasX := c.Params["X"]; !hasX {
					n++
				}
			}
		}
	}
	return n
}
