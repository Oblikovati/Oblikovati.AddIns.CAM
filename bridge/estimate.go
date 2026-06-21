// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"

	"oblikovati.org/cam/bridge/gcode"
)

// defaultRapidRate is the rapid (G0) traverse rate (mm/min) assumed when a tool controller does
// not specify one — only used to estimate cycle time, never emitted.
const defaultRapidRate = 3000.0

// toolChangeSeconds is the fixed time charged per operation for the tool change + spindle
// spin-up the post injects before it.
const toolChangeSeconds = 6.0

// EstimateMinutes estimates a program's cycle time (minutes) from the generated operation
// toolpaths: each move's length divided by its feed (rapids at the controller's rapid rate),
// plus a fixed tool-change allowance per operation. An estimate, not a guarantee — it ignores
// acceleration and dwell, like a quick CAM cycle-time readout.
func EstimateMinutes(results []OperationResult) float64 {
	total := 0.0
	changes := toolChangeAt(results)
	for i, r := range results {
		if changes[i] {
			total += toolChangeSeconds / 60
		}
		total += pathMinutes(r.Path, r.Controller)
	}
	return total
}

// pathMinutes sums the feed time of one operation's toolpath.
func pathMinutes(path gcode.Path, tc ToolController) float64 {
	var px, py, pz float64
	have := false
	minutes := 0.0
	for _, c := range path.Commands {
		nx, ny, nz, moved := nextPosition(c, px, py, pz)
		if moved && have {
			if rate := moveRate(c, tc); rate > 0 {
				minutes += dist3(px, py, pz, nx, ny, nz) / rate
			}
		}
		if moved {
			px, py, pz, have = nx, ny, nz, true
		}
	}
	return minutes
}

// moveRate returns the feed rate (mm/min) for a move: the rapid rate for G0, the move's own F
// when present, else the controller's cutting feed.
func moveRate(c gcode.Command, tc ToolController) float64 {
	if c.Name == "G0" {
		if tc.HorizRapid > 0 {
			return tc.HorizRapid
		}
		return defaultRapidRate
	}
	if f, ok := c.Params["F"]; ok && f > 0 {
		return f
	}
	if tc.HorizFeed > 0 {
		return tc.HorizFeed
	}
	return defaultRapidRate
}

// dist3 is the Euclidean distance between two points.
func dist3(x0, y0, z0, x1, y1, z1 float64) float64 {
	return math.Sqrt((x1-x0)*(x1-x0) + (y1-y0)*(y1-y0) + (z1-z0)*(z1-z0))
}
