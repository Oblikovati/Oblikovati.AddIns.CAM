// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "math"

// CutterShape is the profile family of a milling cutter, which collapses every supported tool to a
// single "radius at a height above the tip" function (see Cutter.radiusAt).
type CutterShape int

const (
	CutterFlat CutterShape = iota // flat endmill: a full-radius cylinder
	CutterBall                    // ball nose: a hemisphere tip then a cylindrical shaft
	CutterCone                    // drill / v-bit / chamfer: a cone that caps at the full radius
)

// Cutter is the swept profile of a tool used by the material-removal simulator: a solid of
// revolution about the vertical axis, described by its radius as a function of height dz above the
// tool tip (the toolpath point). Modelling every cutter this way lets one stamp routine remove the
// material under any tool. All lengths are millimetres.
type Cutter struct {
	Shape   CutterShape
	Radius  float64 // full cutting radius (mm)
	Height  float64 // cutting body height above the tip (mm)
	TanHalf float64 // tan of the cone half-angle (CutterCone only)
}

// radiusAt returns the cutter's radius at height dz above the tip, and 0 outside its cutting body —
// the horizontal reach the cutter removes at that depth.
func (c Cutter) radiusAt(dz float64) float64 {
	if dz < 0 || dz > c.Height {
		return 0
	}
	switch c.Shape {
	case CutterBall:
		if dz < c.Radius {
			return math.Sqrt(c.Radius*c.Radius - (dz-c.Radius)*(dz-c.Radius))
		}
		return c.Radius
	case CutterCone:
		return math.Min(dz*c.TanHalf, c.Radius)
	default: // CutterFlat
		return c.Radius
	}
}

// drillHalfAngleTan is tan of the half-angle of a standard 118°-included-angle twist drill point
// (half-angle 59°), used when a drill bit carries no explicit point angle.
var drillHalfAngleTan = math.Tan(59 * math.Pi / 180)

// CutterFromTool builds the swept profile for a tool bit. A zero cutting-edge height falls back to
// fallbackHeight (the stock height) so the cutter always spans the cut. Tools without an explicit
// point angle default to a 118° drill point / 90° v-bit.
func CutterFromTool(bit ToolBit, fallbackHeight float64) Cutter {
	c := Cutter{Radius: bit.Diameter / 2, Height: cutterHeight(bit.CuttingEdgeHeight, fallbackHeight)}
	switch bit.ShapeType {
	case "ballend", "ballnose":
		c.Shape = CutterBall
	case "drill":
		c.Shape, c.TanHalf = CutterCone, drillHalfAngleTan
	case "vbit", "vcarve", "chamfer", "engraver":
		c.Shape, c.TanHalf = CutterCone, 1 // 90° included angle (45° half)
	default:
		c.Shape = CutterFlat
	}
	return c
}

// cutterHeight prefers the tool's cutting-edge height, falling back to the stock height when unset.
func cutterHeight(edge, fallback float64) float64 {
	if edge > 0 {
		return edge
	}
	return fallback
}
