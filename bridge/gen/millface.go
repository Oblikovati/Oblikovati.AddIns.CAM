// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/geom2d"
)

// Facing patterns. Zigzag sweeps back and forth, linking rows at depth; bidirectional cuts both
// directions but lifts between rows; directional (one-way) cuts every row the same direction for a
// consistent climb, lifting and repositioning between rows; spiral walks concentric rings inward.
const (
	FacePatternZigzag        = "zigzag"
	FacePatternBidirectional = "bidirectional"
	FacePatternDirectional   = "directional"
	FacePatternSpiral        = "spiral"
)

// MillFaceParams configure a face-milling (facing) pass.
type MillFaceParams struct {
	ToolRadius float64 // mm
	StepOver   float64 // fraction of the tool diameter between passes (0..1); 0 → 0.5
	Pattern    string  // FacePattern* (default zigzag); empty + Spiral=true → spiral, for compatibility
	Angle      float64 // raster angle in degrees (0 = rows along X); ignored by the spiral pattern
	Spiral     bool    // legacy flag: clear with the inward spiral instead of a raster (use Pattern instead)
}

// GenerateMillFace clears the top of the stock over the boundary at each depth level. The raster
// patterns ride the region inset by the tool radius at the requested angle; the spiral pattern walks
// concentric rings inward. Ports the facing strategies (zigzag / bidirectional / directional / spiral)
// over an arbitrary raster angle.
func GenerateMillFace(boundary geom2d.Polygon, levels []float64, feeds Feeds, p MillFaceParams) ([]gcode.Command, error) {
	if p.ToolRadius <= 0 {
		return nil, fmt.Errorf("face milling needs a positive tool radius, got %g", p.ToolRadius)
	}
	spacing := stepDistanceFrac(p.StepOver, p.ToolRadius)

	if facingPattern(p) == FacePatternSpiral {
		minX, minY, maxX, maxY := bounds(boundary)
		x0, x1 := minX+p.ToolRadius, maxX-p.ToolRadius
		y0, y1 := minY+p.ToolRadius, maxY-p.ToolRadius
		if x1 <= x0 || y1 <= y0 {
			return nil, fmt.Errorf("face milling: tool radius %g too large for the region (%g×%g)", p.ToolRadius, maxX-minX, maxY-minY)
		}
		rings := faceSpiralRings(insetRect(x0, y0, x1, y1), spacing)
		var cmds []gcode.Command
		for _, z := range levels {
			cmds = append(cmds, walkSpiral(rings, z, feeds, true)...)
		}
		return cmds, nil
	}

	inset, ok := geom2d.Offset(boundary, -p.ToolRadius)
	if !ok || len(inset) < 3 {
		return nil, fmt.Errorf("face milling: tool radius %g too large for the region", p.ToolRadius)
	}
	oneWay := facingPattern(p) == FacePatternDirectional
	linked := facingPattern(p) == FacePatternZigzag

	var cmds []gcode.Command
	for _, z := range levels {
		cmds = append(cmds, facingRasterLevel(inset, z, feeds, p.ToolRadius, spacing, p.Angle, oneWay, linked)...)
	}
	return cmds, nil
}

// facingPattern resolves the requested facing pattern, honouring the explicit Pattern, then the
// legacy Spiral flag, defaulting to zigzag.
func facingPattern(p MillFaceParams) string {
	if p.Pattern != "" {
		return p.Pattern
	}
	if p.Spiral {
		return FacePatternSpiral
	}
	return FacePatternZigzag
}

// facingCut is one raster pass: a cut from a to b within the region.
type facingCut struct {
	a, b geom2d.Point2
}

// facingRasterLevel builds one depth level's raster passes at the requested angle and walks them
// with the chosen traversal. It slices the inset region along each raster line so the passes follow
// the region's true extent (no overhang), then orders them by the pattern.
func facingRasterLevel(inset geom2d.Polygon, z float64, feeds Feeds, toolRadius, spacing, angleDeg float64, oneWay, linked bool) []gcode.Command {
	primary, step := facingAxes(angleDeg)
	origin := inset[0]
	var passes []facingCut
	for _, t := range facingStepPositions(inset, step, origin, toolRadius, spacing) {
		for _, seg := range facingSliceSegments(inset, primary, step, origin, t) {
			passes = append(passes, facingCut{
				a: rasterPoint(origin, primary, step, seg[0], t),
				b: rasterPoint(origin, primary, step, seg[1], t),
			})
		}
	}
	return walkFacingPasses(passes, z, feeds, oneWay, linked)
}

// walkFacingPasses emits the moves for the ordered raster passes. Zigzag (linked) keeps the tool
// down and feeds from one row's end to the next; bidirectional lifts and re-plunges between rows but
// still alternates direction; directional (oneWay) cuts every row in the same direction, lifting and
// repositioning between rows — its rapid return rides clear air above the part, not over just-cut
// stock, so it never trips the rapid-over-stock check.
func walkFacingPasses(passes []facingCut, z float64, feeds Feeds, oneWay, linked bool) []gcode.Command {
	var cmds []gcode.Command
	plunged := false
	for i, c := range passes {
		a, b := c.a, c.b
		if !oneWay && i%2 == 1 {
			a, b = b, a // alternate the cut direction on every other row
		}
		switch {
		case !plunged:
			cmds = append(cmds, plungeAt(a, z, feeds)...)
			plunged = true
		case linked:
			cmds = append(cmds, feedMove(a, feeds.Horiz)) // stay down, link to the next row's start
		default:
			cmds = append(cmds, gcode.NewCommand("G0", map[string]float64{"Z": feeds.ClearanceZ}))
			cmds = append(cmds, plungeAt(a, z, feeds)...)
		}
		cmds = append(cmds, feedMove(b, feeds.Horiz)) // cut the pass
	}
	if plunged {
		cmds = append(cmds, gcode.NewCommand("G0", map[string]float64{"Z": feeds.ClearanceZ}))
	}
	return cmds
}

// insetRect builds the CCW rectangle the facing passes stay within (the region inset by the tool
// radius), used as the outer ring of the spiral facing pattern.
func insetRect(x0, y0, x1, y1 float64) geom2d.Polygon {
	return geom2d.Polygon{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}
}

// faceSpiralRings builds the concentric facing rings for the spiral pattern: the inset rectangle,
// then itself offset inward by the spacing each time until an offset collapses at the centre. The
// rings are linked into one continuous stay-down spiral by walkSpiral, so the cut keeps a
// consistent climb direction and constant engagement — a cleaner facing finish than the zigzag.
func faceSpiralRings(rect geom2d.Polygon, spacing float64) []geom2d.Polygon {
	rings := []geom2d.Polygon{rect.EnsureCCW()}
	if spacing <= 0 {
		return rings
	}
	for d := spacing; ; d += spacing {
		ring, ok := geom2d.Offset(rect, -d)
		if !ok {
			break
		}
		rings = append(rings, ring)
	}
	return rings
}

// stepDistanceFrac converts a step-over fraction of the tool diameter to a distance (mm),
// defaulting to half the diameter.
func stepDistanceFrac(frac, radius float64) float64 {
	if frac <= 0 {
		frac = defaultStepOver
	}
	return frac * 2 * radius
}

// bounds returns the axis-aligned bounding box of a polygon.
func bounds(p geom2d.Polygon) (minX, minY, maxX, maxY float64) {
	minX, minY = p[0].X, p[0].Y
	maxX, maxY = p[0].X, p[0].Y
	for _, v := range p[1:] {
		minX, maxX = minf(minX, v.X), maxf(maxX, v.X)
		minY, maxY = minf(minY, v.Y), maxf(maxY, v.Y)
	}
	return minX, minY, maxX, maxY
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
