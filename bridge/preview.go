// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "oblikovati.org/cam/bridge/gcode"

// PreviewLines is one indexed line list for a toolpath overlay: flat xyz coordinates (cm)
// and index pairs.
type PreviewLines struct {
	Coords  []float64
	Indices []int
}

// ToolpathPreview splits a toolpath into rapid (G0) and cutting (G1/G2/G3) line segments so
// the two can be drawn in distinct colours. It tracks the tool position across moves;
// coordinates are converted from the path's millimetres to the host's centimetres. Arc moves
// (G2/G3) are previewed as straight chords to their endpoint — enough for a path overview.
func ToolpathPreview(path gcode.Path) (rapids, cuts PreviewLines) {
	var px, py, pz float64
	have := false
	for _, c := range path.Commands {
		nx, ny, nz, moved := nextPosition(c, px, py, pz)
		if moved && have {
			if c.Name == "G0" {
				addSegment(&rapids, px, py, pz, nx, ny, nz)
			} else if isCut(c.Name) {
				addSegment(&cuts, px, py, pz, nx, ny, nz)
			}
		}
		if moved {
			px, py, pz, have = nx, ny, nz, true
		}
	}
	return rapids, cuts
}

// nextPosition applies a move command's X/Y/Z to the current position, reporting whether the
// command moved the tool at all.
func nextPosition(c gcode.Command, px, py, pz float64) (x, y, z float64, moved bool) {
	x, y, z = px, py, pz
	if v, ok := c.Params["X"]; ok {
		x, moved = v, true
	}
	if v, ok := c.Params["Y"]; ok {
		y, moved = v, true
	}
	if v, ok := c.Params["Z"]; ok {
		z, moved = v, true
	}
	return x, y, z, moved
}

// isCut reports whether the command name is a cutting move.
func isCut(name string) bool {
	return name == "G1" || name == "G2" || name == "G3"
}

// addSegment appends one segment (two endpoints in cm) to a line list.
func addSegment(lines *PreviewLines, x0, y0, z0, x1, y1, z1 float64) {
	base := len(lines.Coords) / 3
	lines.Coords = append(lines.Coords,
		x0/cmToMM, y0/cmToMM, z0/cmToMM,
		x1/cmToMM, y1/cmToMM, z1/cmToMM,
	)
	lines.Indices = append(lines.Indices, base, base+1)
}
