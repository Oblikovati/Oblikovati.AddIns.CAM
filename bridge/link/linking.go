// SPDX-License-Identifier: GPL-2.0-only

// Package link plans the moves that connect the end of one cut to the start of the next. When the
// straight travel between them clears the part the tool stays down and links directly; otherwise it
// lifts to the lowest collision-free retract height. Whether a travel clears the part is decided by
// an injected CollisionProbe (the host minimum-distance query), so link planning stays host-free
// and fully unit-testable. All lengths are millimetres — the toolpath unit (the probe converts to
// the host's centimetre database unit at the boundary).
package link

import (
	"errors"
	"sort"

	"oblikovati.org/cam/bridge/gcode"
)

// coincideTol is the distance (mm) below which two points count as the same, so a degenerate
// retract/traverse/plunge (zero length) emits no move.
const coincideTol = 1e-4

// DefaultClearance is the part stand-off (mm) a link must keep to count as collision-free — the
// faithful default of the upstream linker (collision_clearance = 1).
const DefaultClearance = 1.0

// ErrNoClearLink is returned when no candidate retract height yields a collision-free travel.
var ErrNoClearLink = errors.New("link: no collision-free travel between the two positions at any retract height")

// CollisionProbe answers how close a straight travel comes to the part. The toolpath layer injects
// the host's body.minimumDistance behind this interface so link planning is testable with a fake.
type CollisionProbe interface {
	// PartClearance returns the minimum distance (mm) from the part to the polyline through pts,
	// the probe widened by toolRadius (mm). 0 means the travel touches or enters the material.
	PartClearance(pts []gcode.Vector3, toolRadius float64) (float64, error)
}

// CheckCollision reports whether a direct move from start to target comes within clearance of the
// part (true = would collide). Ports check_collision: a coincident move never collides; otherwise
// the straight segment is probed and compared to the clearance (with a roughly-equal tolerance so a
// travel exactly at the clearance is treated as clear).
func CheckCollision(start, target gcode.Vector3, probe CollisionProbe, toolRadius, clearance float64) (bool, error) {
	if pointsCoincide(start, target) {
		return false, nil
	}
	clearance = clampClearance(clearance)
	d, err := probe.PartClearance([]gcode.Vector3{start, target}, toolRadius)
	if err != nil {
		return false, err
	}
	return d < clearance && !roughlyEqual(d, clearance), nil
}

// GetLinkingMoves plans the link from the end of one cut (start) to the start of the next (target),
// trying each candidate retract height from lowest to highest and returning the first collision-free
// link as G0 moves. Passing the cut depth as the lowest height lets a clear direct traverse keep the
// tool down (no lift). Ports get_linking_moves. Heights are absolute Z in millimetres.
func GetLinkingMoves(start, target gcode.Vector3, heights []float64, probe CollisionProbe, toolRadius, clearance float64) ([]gcode.Command, error) {
	if pointsCoincide(start, target) {
		return nil, nil
	}
	clearance = clampClearance(clearance)
	ascending := sortedUnique(heights)
	for i := range ascending {
		wire := makeLinkingWire(start, target, ascending[:i+1])
		clear, err := travelClearsPart(wire, probe, toolRadius, clearance)
		if err != nil {
			return nil, err
		}
		if clear {
			return rapidsAlong(wire), nil
		}
	}
	return nil, ErrNoClearLink
}

// makeLinkingWire builds the link polyline: retract from start up to the top height, traverse to
// above the target, then plunge down through the intermediate heights to the target. The top height
// is the last (highest) of the prefix; coincident points are dropped so an equal-depth direct move
// degenerates to a single horizontal traverse (keep-tool-down). Ports make_linking_wire.
func makeLinkingWire(start, target gcode.Vector3, heights []float64) []gcode.Vector3 {
	top := heights[len(heights)-1]
	p1 := gcode.Vector3{X: start.X, Y: start.Y, Z: top}
	p2 := gcode.Vector3{X: target.X, Y: target.Y, Z: top}
	pts := []gcode.Vector3{start}
	pts = appendDistinct(pts, p1)
	pts = appendDistinct(pts, p2)
	// Plunge down through each previously-tried height (highest first), ending at the target depth.
	for i := len(heights) - 2; i >= 0; i-- {
		pts = appendDistinct(pts, gcode.Vector3{X: target.X, Y: target.Y, Z: heights[i]})
	}
	pts = appendDistinct(pts, target)
	return pts
}

// travelClearsPart reports whether the wire's horizontal traverse clears the part. Only the
// horizontal leg is probed — the vertical retract/plunge legs sit over the cut start/target points,
// which are on the toolpath by construction. Ports is_travel_collision_free's _get_hor_edge check.
func travelClearsPart(wire []gcode.Vector3, probe CollisionProbe, toolRadius, clearance float64) (bool, error) {
	a, b, ok := horizontalTraverse(wire)
	if !ok {
		return true, nil
	}
	d, err := probe.PartClearance([]gcode.Vector3{a, b}, toolRadius)
	if err != nil {
		return false, err
	}
	return d >= clearance || roughlyEqual(d, clearance), nil
}

// horizontalTraverse returns the wire's horizontal leg (the two consecutive points at equal Z that
// move in XY) — the segment that risks colliding with the part on the way across.
func horizontalTraverse(wire []gcode.Vector3) (gcode.Vector3, gcode.Vector3, bool) {
	for i := 1; i < len(wire); i++ {
		a, b := wire[i-1], wire[i]
		if roughlyEqual(a.Z, b.Z) && (!roughlyEqual(a.X, b.X) || !roughlyEqual(a.Y, b.Y)) {
			return a, b, true
		}
	}
	return gcode.Vector3{}, gcode.Vector3{}, false
}

// rapidsAlong renders a link polyline as G0 moves (one per leg), emitting only the axes that change
// on each leg so a modal address (an unchanged Z across a traverse) is not repeated — matching how
// the generators write G-code. Ports the cmdsForEdge/G0 loop.
func rapidsAlong(wire []gcode.Vector3) []gcode.Command {
	cmds := make([]gcode.Command, 0, len(wire)-1)
	for i := 1; i < len(wire); i++ {
		prev, p := wire[i-1], wire[i]
		params := map[string]float64{}
		if !roughlyEqual(prev.X, p.X) {
			params["X"] = p.X
		}
		if !roughlyEqual(prev.Y, p.Y) {
			params["Y"] = p.Y
		}
		if !roughlyEqual(prev.Z, p.Z) {
			params["Z"] = p.Z
		}
		cmds = append(cmds, gcode.NewCommand("G0", params))
	}
	return cmds
}

// appendDistinct appends p unless it coincides with the last point already in pts.
func appendDistinct(pts []gcode.Vector3, p gcode.Vector3) []gcode.Vector3 {
	if len(pts) > 0 && pointsCoincide(pts[len(pts)-1], p) {
		return pts
	}
	return append(pts, p)
}

// pointsCoincide reports whether two points are within coincideTol on every axis.
func pointsCoincide(a, b gcode.Vector3) bool {
	return roughlyEqual(a.X, b.X) && roughlyEqual(a.Y, b.Y) && roughlyEqual(a.Z, b.Z)
}

// roughlyEqual reports whether two scalars are within coincideTol.
func roughlyEqual(a, b float64) bool {
	d := a - b
	return d < coincideTol && d > -coincideTol
}

// clampClearance enforces a positive clearance, defaulting a non-positive value to DefaultClearance.
func clampClearance(c float64) float64 {
	if c <= 0 {
		return DefaultClearance
	}
	return c
}

// sortedUnique returns the heights ascending with near-duplicates collapsed.
func sortedUnique(heights []float64) []float64 {
	if len(heights) == 0 {
		return nil
	}
	s := append([]float64(nil), heights...)
	sort.Float64s(s)
	out := s[:1]
	for _, h := range s[1:] {
		if !roughlyEqual(h, out[len(out)-1]) {
			out = append(out, h)
		}
	}
	return out
}
