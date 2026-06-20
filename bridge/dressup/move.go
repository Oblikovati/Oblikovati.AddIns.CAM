// SPDX-License-Identifier: GPL-2.0-only

package dressup

import (
	"math"

	"oblikovati.org/cam/bridge/gcode"
)

// angleTolerance is the radian/coordinate slop below which two angles or positions are
// treated as equal. Mirrors FreeCAD Path.Geom.Tolerance (1e-6), which the dogbone math
// compares tangents against.
const angleTolerance = 1e-6

// move is one planar (XY) toolpath move with tangent directions — the unit the dogbone
// dressup reasons about. A straight move runs begin→end; an arc additionally carries its
// centre and a turn direction (ccw). Ports FreeCAD's PathLanguage MoveStraight / MoveArc;
// only the XY projection matters for corner relief, so Z is ignored.
type move struct {
	begin, end gcode.Vector3
	arc        bool
	center     gcode.Vector3
	ccw        bool
}

// straightMove builds a straight (G1/G0) move from begin to end.
func straightMove(begin, end gcode.Vector3) move {
	return move{begin: begin, end: end}
}

// arcMove builds an arc (G2 cw / G3 ccw) move whose centre is begin + (i, j).
func arcMove(begin, end gcode.Vector3, i, j float64, ccw bool) move {
	return move{begin: begin, end: end, arc: true, center: gcode.Vector3{X: begin.X + i, Y: begin.Y + j}, ccw: ccw}
}

// anglesOfTangents returns the tangent direction (radians) at the move's begin and end. For
// a straight move both equal the segment direction; for an arc each is perpendicular to the
// radius, signed by the turn direction. Ports PathLanguage.MoveStraight/MoveArc.anglesOfTangents.
func (m move) anglesOfTangents() (t0, t1 float64) {
	if !m.arc {
		if roughlySame(m.begin, m.end) {
			return 0, 0
		}
		a := angleOf(sub(m.end, m.begin))
		return a, a
	}
	dir := -math.Pi / 2 // CW
	if m.ccw {
		dir = math.Pi / 2
	}
	s0 := angleOf(sub(m.begin, m.center))
	s1 := angleOf(sub(m.end, m.center))
	return normalizeAngle(s0 + dir), normalizeAngle(s1 + dir)
}

// pathLength returns the move's length in the XY plane (mm) — used to compare adjacent edges
// for the short/long T-bone styles. Ports MoveStraight/MoveArc.pathLength.
func (m move) pathLength() float64 {
	if !m.arc {
		return math.Hypot(m.end.X-m.begin.X, m.end.Y-m.begin.Y)
	}
	return m.arcAngle() * math.Hypot(m.begin.X-m.center.X, m.begin.Y-m.center.Y)
}

// arcAngle returns the opening angle (radians, always positive) the arc subtends, unwinding
// in its turn direction. Ports MoveArc.arcAngle.
func (m move) arcAngle() float64 {
	s0 := angleOf(sub(m.begin, m.center))
	s1 := angleOf(sub(m.end, m.center))
	if !m.ccw {
		for s0 < s1 {
			s0 += 2 * math.Pi
		}
		return s0 - s1
	}
	for s1 < s0 {
		s1 += 2 * math.Pi
	}
	return s1 - s0
}

// angleOf returns the angle of a vector from the +X axis in (-pi, pi]. Ports Path.Geom.getAngle
// (acos against +X, negated for the lower half-plane), which equals atan2(y, x).
func angleOf(v gcode.Vector3) float64 { return math.Atan2(v.Y, v.X) }

// normalizeAngle shifts an angle into [-pi, pi]. Ports Path.Geom.normalizeAngle.
func normalizeAngle(a float64) float64 {
	for a > math.Pi {
		a -= 2 * math.Pi
	}
	for a < -math.Pi {
		a += 2 * math.Pi
	}
	return a
}

// isRoughly reports whether two scalars are equal within angleTolerance. Ports Path.Geom.isRoughly.
func isRoughly(a, b float64) bool { return math.Abs(a-b) <= angleTolerance }

// roughlySame reports whether two XY points coincide within angleTolerance. Ports
// Path.Geom.pointsCoincide (XY only).
func roughlySame(a, b gcode.Vector3) bool {
	return isRoughly(a.X, b.X) && isRoughly(a.Y, b.Y)
}

// sub returns a - b in the XY plane.
func sub(a, b gcode.Vector3) gcode.Vector3 {
	return gcode.Vector3{X: a.X - b.X, Y: a.Y - b.Y}
}
