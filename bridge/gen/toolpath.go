// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/geom2d"
)

// Feeds are the rates and heights a milling operation walks at (millimetres / mm-per-min).
// ClearanceZ is the rapid plane above the part; SafeZ the feed-in transition plane; cutting
// happens at the per-level depth.
type Feeds struct {
	Vert       float64 // plunge feed (mm/min)
	Horiz      float64 // cutting feed in the XY plane (mm/min)
	ClearanceZ float64 // rapid plane (mm)
	SafeZ      float64 // feed-in transition plane (mm)
}

// DepthLevels returns the descending list of cut Z planes from startDepth down to
// finalDepth in steps no larger than stepDown, always finishing exactly on finalDepth. A
// non-positive step collapses to a single pass at finalDepth.
func DepthLevels(startDepth, finalDepth, stepDown float64) []float64 {
	if stepDown <= 0 || startDepth <= finalDepth {
		return []float64{finalDepth}
	}
	var levels []float64
	z := startDepth - stepDown
	for z > finalDepth+1e-9 {
		levels = append(levels, z)
		z -= stepDown
	}
	return append(levels, finalDepth)
}

// walkLoop emits the moves to cut one closed loop at depth z: rapid to the clearance plane,
// rapid over the loop's first point, plunge to z at the vertical feed, then feed around the
// loop at the horizontal feed and close it. The per-loop core of the clearing walk (polyline
// form — offset loops are straight-segment polygons, so only G0/G1 are emitted, no arcs).
func walkLoop(loop geom2d.Polygon, z float64, feeds Feeds) []gcode.Command {
	if len(loop) < 2 {
		return nil
	}
	start := loop[0]
	cmds := []gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"Z": feeds.ClearanceZ}),
		gcode.NewCommand("G0", map[string]float64{"X": start.X, "Y": start.Y}),
		gcode.NewCommand("G0", map[string]float64{"Z": feeds.SafeZ}),
		gcode.NewCommand("G1", map[string]float64{"Z": z, "F": feeds.Vert}),
	}
	for _, pt := range loop[1:] {
		cmds = append(cmds, gcode.NewCommand("G1", map[string]float64{"X": pt.X, "Y": pt.Y, "F": feeds.Horiz}))
	}
	// Close the loop back to the start.
	cmds = append(cmds, gcode.NewCommand("G1", map[string]float64{"X": start.X, "Y": start.Y, "F": feeds.Horiz}))
	cmds = append(cmds, gcode.NewCommand("G0", map[string]float64{"Z": feeds.ClearanceZ}))
	return cmds
}

// walkOpenPath emits the moves to cut one open polyline at depth z: rapid to the clearance
// plane, rapid over the first point, plunge to z, then feed along the path. Unlike walkLoop it
// does not close back to the start — it is the move set for a ring arc that has been clipped
// around an island.
func walkOpenPath(path []geom2d.Point2, z float64, feeds Feeds) []gcode.Command {
	if len(path) < 2 {
		return nil
	}
	cmds := []gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"Z": feeds.ClearanceZ}),
		gcode.NewCommand("G0", map[string]float64{"X": path[0].X, "Y": path[0].Y}),
		gcode.NewCommand("G0", map[string]float64{"Z": feeds.SafeZ}),
		gcode.NewCommand("G1", map[string]float64{"Z": z, "F": feeds.Vert}),
	}
	for _, pt := range path[1:] {
		cmds = append(cmds, gcode.NewCommand("G1", map[string]float64{"X": pt.X, "Y": pt.Y, "F": feeds.Horiz}))
	}
	return append(cmds, gcode.NewCommand("G0", map[string]float64{"Z": feeds.ClearanceZ}))
}

// orient returns the loop wound for the requested cut direction: CCW for climb milling on an
// outside contour, CW for conventional. (Reversing a loop swaps climb/conventional.)
func orient(loop geom2d.Polygon, climb bool) geom2d.Polygon {
	ccw := loop.EnsureCCW()
	if climb {
		return ccw
	}
	return ccw.Reversed()
}
