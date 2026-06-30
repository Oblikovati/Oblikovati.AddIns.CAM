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
// G0/G1/G2/G3 move, with each axis sticky across commands that omit it. Canned drilling/tapping
// cycles are expanded to their plunge motion first, so drilled holes appear in the path.
func toolpathFromGCode(gcodeText string) []gcode.Vector3 {
	points, _ := motionWithKinds(gcodeText)
	return points
}

// motionWithKinds returns the motion polyline (mm) and, aligned to it, whether each point was
// reached by a cutting move (feed) rather than a rapid — so the playback overlay can colour rapids
// and cuts distinctly. feed[i] describes the move into points[i]; feed[0] is the first move's kind.
func motionWithKinds(gcodeText string) ([]gcode.Vector3, []bool) {
	cmds := make([]gcode.Command, 0)
	for _, line := range strings.Split(gcodeText, "\n") {
		cmds = append(cmds, gcode.ParseCommand(line))
	}
	var points []gcode.Vector3
	var feed []bool
	var cur gcode.Vector3
	for _, cmd := range gcode.ExpandCannedCycles(gcode.NewPath(cmds)).Commands {
		cur = applyAxes(cur, cmd)
		if isMotionCommand(cmd.Name) {
			points = append(points, cur)
			feed = append(feed, isFeedMove(cmd.Name))
		}
	}
	return points, feed
}

// isMotionCommand reports whether a G-code name is a rapid or feed move (incl. leading-zero forms).
func isMotionCommand(name string) bool {
	switch name {
	case "G0", "G1", "G2", "G3", "G00", "G01", "G02", "G03":
		return true
	}
	return false
}

// isFeedMove reports whether a motion command cuts (feed) rather than rapids (G0).
func isFeedMove(name string) bool {
	return isMotionCommand(name) && name != "G0" && name != "G00"
}

// segmentLines builds an indexed line list (coords in the host's centimetres) of the polyline
// segments selected by want — segment i joins points[i] and points[i+1]. Each kept segment carries
// its own pair of vertices, so segments of different colours can be drawn from separate calls.
func segmentLines(points []gcode.Vector3, want func(i int) bool) ([]float64, []int) {
	var coords []float64
	var indices []int
	for i := 0; i+1 < len(points); i++ {
		if !want(i) {
			continue
		}
		base := len(coords) / 3
		coords = appendPointCm(appendPointCm(coords, points[i]), points[i+1])
		indices = append(indices, base, base+1)
	}
	return coords, indices
}

// appendPointCm appends a millimetre point to a coordinate stream in the host's centimetres.
func appendPointCm(coords []float64, p gcode.Vector3) []float64 {
	return append(coords, p.X/cmToMM, p.Y/cmToMM, p.Z/cmToMM)
}
