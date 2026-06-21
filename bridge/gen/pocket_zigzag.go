// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/geom2d"
)

// Pocket clearing patterns. The offset pattern walks concentric rings (the default); the zigzag
// pattern sweeps parallel back-and-forth rows across the region, the libarea/FreeCAD ZigZag mode.
const (
	PocketOffset = "offset"
	PocketZigzag = "zigzag"
)

// generateZigzagPocket clears the pocket interior with parallel rows swept back and forth. Each row
// is a horizontal scan line clipped to the tool-reachable region (the boundary inset by wallDist)
// and routed around the islands; consecutive runs are linked at depth when the straight move
// between them stays inside the pocket, otherwise the tool retracts and re-plunges. A finishing
// pass at the walls is appended when one was requested.
func generateZigzagPocket(boundary geom2d.Polygon, levels []float64, feeds Feeds, p PocketParams, wallDist float64) ([]gcode.Command, error) {
	inset, ok := geom2d.Offset(boundary, -wallDist)
	if !ok {
		return nil, pocketTooSmallErr(p, boundary)
	}
	keepouts := grownIslands(p.Islands, wallDist)
	finishRings, finishKeepouts := pocketFinishPass(boundary, p)

	minX, _, maxX, _ := bounds(inset)
	rows := scanRows(inset, p.stepDistance())

	var cmds []gcode.Command
	for _, z := range levels {
		cmds = append(cmds, zigzagLevel(inset, keepouts, minX, maxX, rows, z, feeds)...)
		cmds = append(cmds, walkPocketRings(finishRings, finishKeepouts, z, feeds, p.Climb)...)
	}
	return cmds, nil
}

// scanRows returns the Y positions of the scan lines across the inset region, spaced by no more
// than spacing and inset half a step from the extremes so the first/last rows are not on the wall.
func scanRows(inset geom2d.Polygon, spacing float64) []float64 {
	_, minY, _, maxY := bounds(inset)
	if spacing <= 0 || maxY-minY <= spacing {
		return []float64{(minY + maxY) / 2}
	}
	n := int((maxY-minY)/spacing) + 1
	step := (maxY - minY) / float64(n)
	rows := make([]float64, 0, n)
	for y := minY + step/2; y < maxY; y += step {
		rows = append(rows, y)
	}
	return rows
}

// zigzagLevel builds one depth level's runs (boustrophedon-ordered) and walks them, linking
// consecutive runs at depth where the connecting move stays in the pocket.
func zigzagLevel(inset geom2d.Polygon, islands []geom2d.Polygon, minX, maxX float64, rows []float64, z float64, feeds Feeds) []gcode.Command {
	var ordered [][]geom2d.Point2
	for i, y := range rows {
		runs := rowRuns(inset, islands, minX, maxX, y)
		if i%2 == 1 {
			runs = reverseRuns(runs)
		}
		ordered = append(ordered, runs...)
	}
	return walkLinkedRuns(ordered, inset, islands, z, feeds)
}

// rowRuns clips one scan line at height y to the interior runs clear of every island.
func rowRuns(inset geom2d.Polygon, islands []geom2d.Polygon, minX, maxX, y float64) [][]geom2d.Point2 {
	line := []geom2d.Point2{{X: minX - 1, Y: y}, {X: maxX + 1, Y: y}}
	runs := geom2d.ClipInside(line, inset)
	for _, isl := range islands {
		var next [][]geom2d.Point2
		for _, r := range runs {
			next = append(next, geom2d.ClipOutside(r, isl)...)
		}
		runs = next
	}
	return runs
}

// reverseRuns reverses both the order of the runs and the points within each, so an odd row sweeps
// right-to-left for the back-and-forth motion.
func reverseRuns(runs [][]geom2d.Point2) [][]geom2d.Point2 {
	out := make([][]geom2d.Point2, 0, len(runs))
	for i := len(runs) - 1; i >= 0; i-- {
		r := runs[i]
		rev := make([]geom2d.Point2, len(r))
		for j := range r {
			rev[j] = r[len(r)-1-j]
		}
		out = append(out, rev)
	}
	return out
}

// walkLinkedRuns cuts the ordered runs: plunge at the first, feed along each, and between runs
// either feed straight across at depth (when the link stays inside the pocket) or retract, rapid,
// and re-plunge.
func walkLinkedRuns(runs [][]geom2d.Point2, inset geom2d.Polygon, islands []geom2d.Polygon, z float64, feeds Feeds) []gcode.Command {
	var cmds []gcode.Command
	plunged := false
	var pos geom2d.Point2
	for _, run := range runs {
		if len(run) < 2 {
			continue
		}
		if !plunged {
			cmds = append(cmds, plungeAt(run[0], z, feeds)...)
			plunged = true
		} else if connectorClear(pos, run[0], inset, islands) {
			cmds = append(cmds, feedMove(run[0], feeds.Horiz)) // stay down across the link
		} else {
			cmds = append(cmds, gcode.NewCommand("G0", map[string]float64{"Z": feeds.ClearanceZ}))
			cmds = append(cmds, plungeAt(run[0], z, feeds)...)
		}
		for _, pt := range run[1:] {
			cmds = append(cmds, feedMove(pt, feeds.Horiz))
		}
		pos = run[len(run)-1]
	}
	if plunged {
		cmds = append(cmds, gcode.NewCommand("G0", map[string]float64{"Z": feeds.ClearanceZ}))
	}
	return cmds
}

// plungeAt rapids over a point at the clearance plane and plunges to depth z at the vertical feed.
func plungeAt(pt geom2d.Point2, z float64, feeds Feeds) []gcode.Command {
	return []gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"Z": feeds.ClearanceZ}),
		gcode.NewCommand("G0", map[string]float64{"X": pt.X, "Y": pt.Y}),
		gcode.NewCommand("G0", map[string]float64{"Z": feeds.SafeZ}),
		gcode.NewCommand("G1", map[string]float64{"Z": z, "F": feeds.Vert}),
	}
}

// connectorClear reports whether the straight move a→b stays inside the pocket: it must not cross
// out through the inset boundary nor cross into any island. The runs' endpoints are already on or
// inside the inset and outside the islands, so "crosses nothing" means the whole link stays clear —
// and a link running along a wall (collinear with the boundary) is allowed, so consecutive rows
// can be joined at depth along the inset edge.
func connectorClear(a, b geom2d.Point2, inset geom2d.Polygon, islands []geom2d.Polygon) bool {
	if inset.SegmentCrosses(a, b) {
		return false
	}
	for _, isl := range islands {
		if isl.SegmentCrosses(a, b) || isl.Contains(midpoint(a, b)) {
			return false
		}
	}
	return true
}

// midpoint returns the point halfway between a and b.
func midpoint(a, b geom2d.Point2) geom2d.Point2 {
	return geom2d.Point2{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
}
