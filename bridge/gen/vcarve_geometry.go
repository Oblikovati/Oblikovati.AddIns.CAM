// SPDX-License-Identifier: GPL-2.0-only

package gen

import "math"

// vcarveGeometry holds the precomputed V-carve depth limits, derived once from the tool and the
// Z range. A V-bit cuts deeper the wider the clearance under it: a medial point with clearance
// (max inscribed circle radius) c is cut at start − c·scale, clamped to the depth limit. Faithful
// port of the reference workbench's _Geometry.
type vcarveGeometry struct {
	start        float64 // top Z the carve starts from (shifted up by the tip radius for a flat-tipped bit)
	stop         float64 // deepest Z the carve may reach (the tool's max depth or the user final depth)
	scale        float64 // depth per unit clearance = 1/tan(halfAngle)
	stepDown     float64 // max depth increment per pass (0 → single pass to stop)
	stepDownPass int     // 1-based pass number, raised between roughing passes
	offset       float64 // finishing-pass override added to every depth (e.g. −0.1 to clean fuzz)
}

// vcarveGeometryFromTool derives the depth limits from the V-bit (included angle, full and tip
// diameter) and the Z range. rMax sets the deepest the full-diameter bit reaches; a non-zero tip
// radius shifts the whole carve up by rMin·scale so the engraved width stays correct. Exact port of
// _Geometry.FromTool.
func vcarveGeometryFromTool(diameter, cuttingEdgeAngleDeg, tipDiameter, zStart, zFinal, zStepDown float64) vcarveGeometry {
	rMax := diameter / 2
	rMin := tipDiameter / 2
	toolAngle := math.Tan(cuttingEdgeAngleDeg / 2 * math.Pi / 180)
	zScale := 1.0 / toolAngle
	zStop := zStart - rMax*zScale
	zOff := rMin * zScale
	return vcarveGeometry{
		start:        zStart + zOff,
		stop:         math.Max(zStop+zOff, zFinal),
		scale:        zScale,
		stepDown:     zStepDown,
		stepDownPass: 1,
	}
}

// maximumDepth is the deepest Z allowed for the current pass: the stop plane, or the start lifted by
// the accumulated step-down when step-down roughing is on. Exact port of _Geometry.maximumDepth.
func (g vcarveGeometry) maximumDepth() float64 {
	if g.stepDown == 0 {
		return g.stop
	}
	return math.Max(g.stop, g.start-float64(g.stepDownPass)*g.stepDown)
}

// depthForClearance returns the cut Z for a medial point whose maximum inscribed circle has radius
// mic (mm): start − mic·scale, clamped to the pass depth limit, plus the finishing offset. Exact
// port of _calculate_depth (including its round-to-4-decimals on the scaled clearance).
func (g vcarveGeometry) depthForClearance(mic float64) float64 {
	depth := g.start - round4(mic*g.scale)
	return math.Max(depth, g.maximumDepth()) + g.offset
}

// round4 rounds to four decimal places, matching the reference workbench's round(x, 4).
func round4(x float64) float64 {
	return math.Round(x*1e4) / 1e4
}
