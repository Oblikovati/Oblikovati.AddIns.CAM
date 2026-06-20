// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"fmt"
	"math"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/geom2d"
)

// TrochoidalParams configure trochoidal milling: instead of a full-width contour pass, the tool
// follows the path as a chain of overlapping circular loops, so radial engagement stays low and
// the controller can run a high feed (good for deep slots / hardened stock). LoopRadius is each
// loop's radius; Advance is how far the loop centre steps along the path between loops (smaller
// than 2·LoopRadius so the loops overlap).
type TrochoidalParams struct {
	ToolRadius float64 // mm — compensates the boundary to the cut side
	LoopRadius float64 // mm — radius of each trochoidal loop
	Advance    float64 // mm — centre spacing along the path between loops
	Side       string  // SideOutside | SideInside | SideOn
	Climb      bool    // climb (G3) vs conventional (G2) loops
}

// GenerateTrochoidal walks the (radius-compensated) boundary as a chain of overlapping full
// circles — one loop of LoopRadius at every Advance along the path — at each depth level. Each
// loop is two 180° arcs about its centre, joined to the next by a short feed move. This is the
// add-in's trochoidal-milling toolpath (FreeCAD's Path trochoid dressup applied along a contour).
func GenerateTrochoidal(boundary geom2d.Polygon, levels []float64, feeds Feeds, p TrochoidalParams) ([]gcode.Command, error) {
	if p.LoopRadius <= 0 {
		return nil, fmt.Errorf("trochoidal milling needs a positive loop radius, got %g", p.LoopRadius)
	}
	if p.Advance <= 0 {
		return nil, fmt.Errorf("trochoidal milling needs a positive advance, got %g", p.Advance)
	}
	if len(boundary) < 3 {
		return nil, fmt.Errorf("trochoidal boundary needs at least 3 points, got %d", len(boundary))
	}
	centerPath, err := compensate(boundary, ProfileParams{ToolRadius: nonNeg(p.ToolRadius), Side: p.Side})
	if err != nil {
		return nil, err
	}
	centers := resampleClosed(orient(centerPath, p.Climb), p.Advance)
	if len(centers) == 0 {
		return nil, fmt.Errorf("trochoidal path has no loop centres (advance %g too large)", p.Advance)
	}
	arc := "G2"
	if p.Climb {
		arc = "G3"
	}
	var cmds []gcode.Command
	for _, z := range levels {
		cmds = append(cmds, trochoidalPass(centers, p.LoopRadius, z, feeds, arc)...)
	}
	return cmds, nil
}

// trochoidalPass emits one depth level: rapid in and plunge at the first loop's start, then a
// full circle at each centre joined by short feed moves, then retract.
func trochoidalPass(centers []geom2d.Point2, r, z float64, feeds Feeds, arc string) []gcode.Command {
	start := geom2d.Point2{X: centers[0].X + r, Y: centers[0].Y} // east point of the first loop
	cmds := []gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"Z": feeds.ClearanceZ}),
		gcode.NewCommand("G0", map[string]float64{"X": start.X, "Y": start.Y}),
		gcode.NewCommand("G0", map[string]float64{"Z": feeds.SafeZ}),
		gcode.NewCommand("G1", map[string]float64{"Z": z, "F": feeds.Vert}),
	}
	for i, c := range centers {
		if i > 0 {
			cmds = append(cmds, gcode.NewCommand("G1", map[string]float64{"X": c.X + r, "Y": c.Y, "F": feeds.Horiz}))
		}
		cmds = append(cmds, loopArcs(c, r, arc, feeds.Horiz)...)
	}
	return append(cmds, gcode.NewCommand("G0", map[string]float64{"Z": feeds.ClearanceZ}))
}

// loopArcs are the two 180° arcs of one full circle of radius r about centre c, starting and
// ending at c's east point.
func loopArcs(c geom2d.Point2, r float64, arc string, feed float64) []gcode.Command {
	west := map[string]float64{"X": c.X - r, "Y": c.Y, "I": -r, "J": 0, "F": feed}
	east := map[string]float64{"X": c.X + r, "Y": c.Y, "I": r, "J": 0, "F": feed}
	return []gcode.Command{gcode.NewCommand(arc, west), gcode.NewCommand(arc, east)}
}

// resampleClosed returns points evenly spaced by `spacing` (mm) along the closed polygon's
// perimeter, starting at the first vertex — the loop centres the trochoid marches through.
func resampleClosed(poly geom2d.Polygon, spacing float64) []geom2d.Point2 {
	n := len(poly)
	if n < 2 || spacing <= 0 {
		return poly
	}
	pts := []geom2d.Point2{poly[0]}
	acc, target := 0.0, spacing
	for i := 0; i < n; i++ {
		a, b := poly[i], poly[(i+1)%n]
		segLen := math.Hypot(b.X-a.X, b.Y-a.Y)
		for segLen > 0 && acc+segLen >= target {
			t := (target - acc) / segLen
			pts = append(pts, geom2d.Point2{X: a.X + (b.X-a.X)*t, Y: a.Y + (b.Y-a.Y)*t})
			target += spacing
		}
		acc += segLen
	}
	return pts
}

// nonNeg clamps a value to be at least zero (a zero tool radius means run loop centres on the
// boundary).
func nonNeg(v float64) float64 {
	if v < 0 {
		return 0
	}
	return v
}
