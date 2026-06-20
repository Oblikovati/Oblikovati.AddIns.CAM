// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/geom2d"
)

// MillFaceParams configure a face-milling (facing) pass.
type MillFaceParams struct {
	ToolRadius float64 // mm
	StepOver   float64 // fraction of the tool diameter between raster rows (0..1); 0 → 0.5
}

// GenerateMillFace clears the top of the stock over the boundary's bounding region with a
// back-and-forth raster (zigzag) at each depth level — the simplest facing pattern. The
// raster is inset by the tool radius so the tool stays within the region. Ports the role of
// FreeCAD's Path/Op/MillFace (the raster-clearing core). Rows run along X, stepping in Y.
func GenerateMillFace(boundary geom2d.Polygon, levels []float64, feeds Feeds, p MillFaceParams) ([]gcode.Command, error) {
	if p.ToolRadius <= 0 {
		return nil, fmt.Errorf("face milling needs a positive tool radius, got %g", p.ToolRadius)
	}
	minX, minY, maxX, maxY := bounds(boundary)
	x0, x1 := minX+p.ToolRadius, maxX-p.ToolRadius
	y0, y1 := minY+p.ToolRadius, maxY-p.ToolRadius
	if x1 <= x0 || y1 <= y0 {
		return nil, fmt.Errorf("face milling: tool radius %g too large for the region (%g×%g)", p.ToolRadius, maxX-minX, maxY-minY)
	}
	rows := passLines(y0, y1, stepDistanceFrac(p.StepOver, p.ToolRadius))

	var cmds []gcode.Command
	for _, z := range levels {
		cmds = append(cmds, rasterLevel(x0, x1, rows, z, feeds)...)
	}
	return cmds, nil
}

// rasterLevel emits one depth level's zigzag: rapid in, plunge, then alternate-direction X
// cuts with a Y step between rows, then retract.
func rasterLevel(x0, x1 float64, rows []float64, z float64, feeds Feeds) []gcode.Command {
	cmds := []gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"Z": feeds.ClearanceZ}),
		gcode.NewCommand("G0", map[string]float64{"X": x0, "Y": rows[0]}),
		gcode.NewCommand("G0", map[string]float64{"Z": feeds.SafeZ}),
		gcode.NewCommand("G1", map[string]float64{"Z": z, "F": feeds.Vert}),
	}
	cur := x0
	for i, y := range rows {
		target := x1
		if cur == x1 {
			target = x0
		}
		cmds = append(cmds, gcode.NewCommand("G1", map[string]float64{"X": target, "Y": y, "F": feeds.Horiz}))
		cur = target
		if i < len(rows)-1 {
			cmds = append(cmds, gcode.NewCommand("G1", map[string]float64{"Y": rows[i+1], "F": feeds.Horiz}))
		}
	}
	return append(cmds, gcode.NewCommand("G0", map[string]float64{"Z": feeds.ClearanceZ}))
}

// passLines returns the row positions from lo to hi spaced by no more than spacing, always
// including both ends (a single row at the midpoint when the band is thinner than spacing).
func passLines(lo, hi, spacing float64) []float64 {
	if spacing <= 0 || hi-lo <= spacing {
		return []float64{(lo + hi) / 2}
	}
	n := int((hi-lo)/spacing) + 1
	step := (hi - lo) / float64(n)
	rows := make([]float64, 0, n+1)
	for y := lo; y < hi-1e-9; y += step {
		rows = append(rows, y)
	}
	return append(rows, hi)
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
