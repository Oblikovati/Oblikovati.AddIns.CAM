// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
)

// SurfaceFinishParams configure a parallel 3D finishing pass built from drop-cutter rows.
type SurfaceFinishParams struct {
	ClearanceZ float64 // rapid/retract plane above the part (mm)
	Zigzag     bool    // alternate row direction (true) vs one-way passes (false)
}

// GenerateSurfaceFinish turns drop-cutter cutter-location rows (each a scan line's points
// already riding the surface, mm) into a parallel finishing toolpath: per row it retracts,
// rapids over the first point, plunges, then feeds through the row. Zigzag reverses alternate
// rows so consecutive passes start adjacent (less rapid travel). This is the toolpath-shaping
// half of 3D surfacing; OpenCAMLib (bridge/ocl) computes the surface-following Z.
func GenerateSurfaceFinish(rows [][]gcode.Vector3, feeds Feeds, p SurfaceFinishParams) ([]gcode.Command, error) {
	var cmds []gcode.Command
	emitted := 0
	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		pts := orientRow(row, p.Zigzag && emitted%2 == 1)
		cmds = append(cmds, leadInSurface(pts[0], feeds, p.ClearanceZ)...)
		for _, pt := range pts[1:] {
			cmds = append(cmds, gcode.NewCommand("G1", map[string]float64{"X": pt.X, "Y": pt.Y, "Z": pt.Z, "F": feeds.Horiz}))
		}
		emitted++
	}
	if emitted == 0 {
		return nil, fmt.Errorf("3D finish produced no usable rows (need at least one scan line with 2+ points)")
	}
	return append(cmds, gcode.NewCommand("G0", map[string]float64{"Z": p.ClearanceZ})), nil
}

// leadInSurface retracts to clearance, rapids over the row's first point, and plunges to it.
func leadInSurface(first gcode.Vector3, feeds Feeds, clearanceZ float64) []gcode.Command {
	return []gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"Z": clearanceZ}),
		gcode.NewCommand("G0", map[string]float64{"X": first.X, "Y": first.Y}),
		gcode.NewCommand("G1", map[string]float64{"Z": first.Z, "F": feeds.Vert}),
	}
}

// orientRow returns the row reversed when this pass runs the opposite direction (zigzag).
func orientRow(row []gcode.Vector3, flip bool) []gcode.Vector3 {
	if !flip {
		return row
	}
	out := make([]gcode.Vector3, len(row))
	for i, p := range row {
		out[len(row)-1-i] = p
	}
	return out
}
