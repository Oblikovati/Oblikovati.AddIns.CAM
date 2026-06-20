// SPDX-License-Identifier: GPL-2.0-only

package dressup

import (
	"math"

	"oblikovati.org/cam/bridge/gcode"
)

// Dogbone styles: the direction a corner-relief bone is cut so an end mill of finite radius
// can clear an internal corner. Mirrors FreeCAD's DogboneII styles.
const (
	StyleDogbone   = "dogbone"          // along the corner bisector (shortest relief)
	StyleTBoneH    = "tbone_horizontal" // along the X axis
	StyleTBoneV    = "tbone_vertical"   // along the Y axis
	StyleTBoneLong = "tbone_long"       // along the longer adjacent edge
	StyleTBoneShrt = "tbone_short"      // along the shorter adjacent edge
)

// DogboneParams configure the dogbone dressup. Style picks the bone direction; Length is the
// bone reach (mm, typically the tool radius); MinAngle is the deflection threshold (radians)
// below which a corner is too shallow to bother relieving; Side selects which corners get
// bones — "left" (inside of a CCW loop), "right", or "both".
type DogboneParams struct {
	Style    string
	Length   float64
	MinAngle float64
	Side     string
}

// Dogbone side selectors.
const (
	SideLeft  = "left"
	SideRight = "right"
	SideBoth  = "both"
)

// kink is the corner where one move meets the next: the end tangent of m0 against the begin
// tangent of m1, with the deflection (signed turn) between them. Ports DogboneII.Kink.
type kink struct {
	m0, m1 move
	t0, t1 float64
	defl   float64
}

// newKink computes the tangents and deflection at the junction of two moves.
func newKink(m0, m1 move) kink {
	_, t0 := m0.anglesOfTangents()
	t1, _ := m1.anglesOfTangents()
	defl := 0.0
	if !isRoughly(t0, t1) {
		defl = normalizeAngle(t1 - t0)
	}
	return kink{m0: m0, m1: m1, t0: t0, t1: t1, defl: defl}
}

// position returns the corner point (the shared end of m0 / begin of m1).
func (k kink) position() gcode.Vector3 { return k.m0.end }

// goesRight reports a right (clockwise) turn at the corner.
func (k kink) goesRight() bool { return k.defl < 0 }

// normAngle returns the angle of the corner bisector pointing into the relief direction.
// Ports DogboneII.Kink.normAngle: the perpendicular to the average tangent, turned toward
// the side the two tangents' ordering selects.
func (k kink) normAngle() float64 {
	if k.t0 > k.t1 {
		return normalizeAngle((k.t0 + k.t1 + math.Pi) / 2)
	}
	return normalizeAngle((k.t0 + k.t1 - math.Pi) / 2)
}

// boneAngle returns the bone direction (radians) for a style at a kink. Ports the angle()
// methods of DogboneII's Generator subclasses.
func boneAngle(style string, k kink) float64 {
	switch style {
	case StyleTBoneH:
		if math.Abs(k.normAngle()) > math.Pi/2 {
			return -math.Pi
		}
		return 0
	case StyleTBoneV:
		if k.normAngle() > 0 {
			return math.Pi / 2
		}
		return -math.Pi / 2
	case StyleTBoneShrt:
		return edgeBoneAngle(k, k.m0.pathLength() < k.m1.pathLength())
	case StyleTBoneLong:
		return edgeBoneAngle(k, k.m0.pathLength() > k.m1.pathLength())
	default: // StyleDogbone
		return k.normAngle()
	}
}

// edgeBoneAngle aligns the bone with one adjacent edge: useOnM0 picks m0's edge (turned 90°
// toward the corner), otherwise m1's. Shared by the short/long T-bone styles.
func edgeBoneAngle(k kink, useOnM0 bool) float64 {
	rot := -math.Pi / 2
	if k.goesRight() {
		rot = math.Pi / 2
	}
	if useOnM0 {
		return normalizeAngle(k.t0 + rot)
	}
	return normalizeAngle(k.t1 + rot)
}

// generateBone builds the two-move relief bone at a kink: a move out by Length along angle,
// then a move back to the corner. Each move carries only the axis (axes) it changes, matching
// FreeCAD's generate_bone, so a tangent (axis-aligned) bone stays a single-axis move.
func generateBone(k kink, length, angle float64) (in, out gcode.Command) {
	dx := length * math.Cos(angle)
	dy := length * math.Sin(angle)
	p0 := k.position()
	switch {
	case isRoughly(0, dx): // vertical bone
		return g1(map[string]float64{"Y": p0.Y + dy}), g1(map[string]float64{"Y": p0.Y})
	case isRoughly(0, dy): // horizontal bone
		return g1(map[string]float64{"X": p0.X + dx}), g1(map[string]float64{"X": p0.X})
	default:
		return g1(map[string]float64{"X": p0.X + dx, "Y": p0.Y + dy}), g1(map[string]float64{"X": p0.X, "Y": p0.Y})
	}
}

// g1 wraps a parameter map as a G1 feed move.
func g1(params map[string]float64) gcode.Command { return gcode.NewCommand("G1", params) }

// qualifies reports whether a kink's deflection clears the threshold on the selected side.
// Ports DogboneII.findDogboneKinks (positive threshold keeps left turns, negative keeps right).
func qualifies(k kink, p DogboneParams) bool {
	switch p.Side {
	case SideRight:
		return k.defl < -p.MinAngle
	case SideLeft:
		return k.defl > p.MinAngle
	default: // both
		return math.Abs(k.defl) > p.MinAngle
	}
}
