// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
)

// LevelLoops are the constant-Z contour loops machined at one cutting height — the output of
// slicing the cutter-location surface at Z. Each loop is its closed XY path at height Z (mm).
type LevelLoops struct {
	Z     float64
	Loops [][]gcode.Vector3
}

// WaterlineParams configure z-level (waterline) finishing.
type WaterlineParams struct {
	ClearanceZ float64 // rapid/retract plane above the part (mm)
}

// GenerateWaterline emits z-level finishing passes: for each level (the caller orders them
// top-to-bottom), every contour loop is rapid-approached at clearance, plunged to the level,
// and cut around. This is the toolpath-shaping half of waterline finishing; the constant-Z
// loops come from contouring the drop-cutter surface (Heightfield.Contour).
func GenerateWaterline(levels []LevelLoops, feeds Feeds, p WaterlineParams) ([]gcode.Command, error) {
	cmds := []gcode.Command{gcode.NewCommand("G0", map[string]float64{"Z": p.ClearanceZ})}
	cut := 0
	for _, lvl := range levels {
		for _, loop := range lvl.Loops {
			if len(loop) < 2 {
				continue
			}
			cmds = append(cmds, waterlineLoop(loop, lvl.Z, feeds, p.ClearanceZ)...)
			cut++
		}
	}
	if cut == 0 {
		return nil, fmt.Errorf("waterline produced no loops (no level intersected the surface)")
	}
	return append(cmds, gcode.NewCommand("G0", map[string]float64{"Z": p.ClearanceZ})), nil
}

// waterlineLoop cuts one closed contour at height z: rapid over the start, plunge, feed around,
// and close back to the start.
func waterlineLoop(loop []gcode.Vector3, z float64, feeds Feeds, clearanceZ float64) []gcode.Command {
	start := loop[0]
	cmds := []gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"Z": clearanceZ}),
		gcode.NewCommand("G0", map[string]float64{"X": start.X, "Y": start.Y}),
		gcode.NewCommand("G1", map[string]float64{"Z": z, "F": feeds.Vert}),
	}
	for _, pt := range loop[1:] {
		cmds = append(cmds, gcode.NewCommand("G1", map[string]float64{"X": pt.X, "Y": pt.Y, "F": feeds.Horiz}))
	}
	return append(cmds, gcode.NewCommand("G1", map[string]float64{"X": start.X, "Y": start.Y, "F": feeds.Horiz}))
}
