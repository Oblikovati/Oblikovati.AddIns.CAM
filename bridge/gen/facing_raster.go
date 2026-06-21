// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"math"
	"sort"

	"oblikovati.org/cam/bridge/geom2d"
)

// The angled-raster core for facing, a port of the reference workbench's facing_common helpers. A
// raster runs along the "primary" axis and steps along the perpendicular "step" axis; both come from
// the requested angle, so the rows can lie at any orientation, not just along X. Positions are
// expressed as (s, t) coordinates in that rotated frame — s along primary, t along step — and mapped
// back to XY with rasterPoint.

// facingAxes returns the primary (cut) and step unit vectors for a raster at angleDeg degrees:
// primary points along the angle, step is its +90° normal. Port of unit_vectors_from_angle.
func facingAxes(angleDeg float64) (primary, step geom2d.Point2) {
	rad := angleDeg * math.Pi / 180
	primary = geom2d.Point2{X: math.Cos(rad), Y: math.Sin(rad)}
	step = geom2d.Point2{X: -math.Sin(rad), Y: math.Cos(rad)}
	return primary, step
}

// vdot is the dot product of the vector (p − origin) with axis.
func vdot(p, origin, axis geom2d.Point2) float64 {
	return (p.X-origin.X)*axis.X + (p.Y-origin.Y)*axis.Y
}

// projectBounds returns the min and max projection of the polygon's vertices onto axis, measured
// from origin. Port of project_bounds.
func projectBounds(poly geom2d.Polygon, axis, origin geom2d.Point2) (lo, hi float64) {
	lo, hi = math.Inf(1), math.Inf(-1)
	for _, v := range poly {
		t := vdot(v, origin, axis)
		lo, hi = math.Min(lo, t), math.Max(hi, t)
	}
	return lo, hi
}

// facingStepPositions returns the step coordinates of the raster lines across the region. The first
// pass engages one stepover of the tool so its engaged edge meets the region boundary, then it steps
// by the stepover until the tool's engaged edge clears the far boundary. Port of generate_t_values
// (with the step-over already resolved to a distance in millimetres).
func facingStepPositions(poly geom2d.Polygon, step, origin geom2d.Point2, toolRadius, stepover float64) []float64 {
	minT, maxT := projectBounds(poly, step, origin)
	t := minT - toolRadius + stepover
	if stepover <= 0 {
		return []float64{t}
	}
	tEnd := maxT + toolRadius - stepover
	var out []float64
	for t <= tEnd+1e-9 {
		out = append(out, t)
		t += stepover
	}
	if len(out) == 0 { // region thinner than one stepover: a single centred pass
		out = []float64{(minT + maxT) / 2}
	}
	return out
}

// facingSliceSegments intersects the region polygon with the raster line at step coordinate t and
// returns the interior (sStart, sEnd) intervals along the primary axis, sorted and paired. Port of
// slice_wire_segments: it crosses each edge of the polygon, projects the crossing onto primary, and
// pairs successive crossings into inside segments — so a convex region yields one segment and a
// region pinched by an island yields several.
func facingSliceSegments(poly geom2d.Polygon, primary, step, origin geom2d.Point2, t float64) [][2]float64 {
	var crossings []float64
	n := len(poly)
	for i := 0; i < n; i++ {
		a, b := poly[i], poly[(i+1)%n]
		ta := vdot(a, origin, step)
		tb := vdot(b, origin, step)
		denom := tb - ta
		if math.Abs(denom) < 1e-12 { // edge parallel to the raster line
			continue
		}
		da, db := ta-t, tb-t
		if !((da <= 0 && db >= 0) || (da >= 0 && db <= 0)) {
			continue
		}
		u := math.Max(0, math.Min(1, (t-ta)/denom))
		x := a.X + u*(b.X-a.X)
		y := a.Y + u*(b.Y-a.Y)
		crossings = append(crossings, primary.X*(x-origin.X)+primary.Y*(y-origin.Y))
	}
	sort.Float64s(crossings)
	var segs [][2]float64
	for i := 0; i+1 < len(crossings); i += 2 {
		if crossings[i+1] > crossings[i]+1e-9 {
			segs = append(segs, [2]float64{crossings[i], crossings[i+1]})
		}
	}
	return segs
}

// rasterPoint maps an (s, t) coordinate in the rotated raster frame back to an XY point:
// origin + primary·s + step·t.
func rasterPoint(origin, primary, step geom2d.Point2, s, t float64) geom2d.Point2 {
	return geom2d.Point2{
		X: origin.X + primary.X*s + step.X*t,
		Y: origin.Y + primary.Y*s + step.Y*t,
	}
}
