// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"strings"

	"oblikovati.org/cam/bridge/gcode"
)

// Toolpath extraction for the simulator: the tool's motion path is recovered from the posted
// G-code by walking the rapid/feed motion commands and tracking the (sticky) tool position. Arcs
// are taken to their endpoint — enough for a path preview/playback (not material removal).

// toolpathFromGCode returns the tool's motion polyline (mm) from a posted program: one point per
// G0/G1/G2/G3 move, with each axis sticky across commands that omit it.
func toolpathFromGCode(gcodeText string) []gcode.Vector3 {
	var points []gcode.Vector3
	var cur gcode.Vector3
	for _, line := range strings.Split(gcodeText, "\n") {
		cmd := gcode.ParseCommand(line)
		if x, ok := cmd.Params["X"]; ok {
			cur.X = x
		}
		if y, ok := cmd.Params["Y"]; ok {
			cur.Y = y
		}
		if z, ok := cmd.Params["Z"]; ok {
			cur.Z = z
		}
		if isMotionCommand(cmd.Name) {
			points = append(points, cur)
		}
	}
	return points
}

// isMotionCommand reports whether a G-code name is a rapid or feed move (incl. leading-zero forms).
func isMotionCommand(name string) bool {
	switch name {
	case "G0", "G1", "G2", "G3", "G00", "G01", "G02", "G03":
		return true
	}
	return false
}

// polylineLines builds an indexed line list (coords in the host's centimetres) from a millimetre
// polyline — the line-strip the simulator draws for the traced/remaining toolpath.
func polylineLines(points []gcode.Vector3) ([]float64, []int) {
	coords := make([]float64, 0, len(points)*3)
	for _, p := range points {
		coords = append(coords, p.X/cmToMM, p.Y/cmToMM, p.Z/cmToMM)
	}
	var indices []int
	for i := 0; i+1 < len(points); i++ {
		indices = append(indices, i, i+1)
	}
	return coords, indices
}
