// SPDX-License-Identifier: GPL-2.0-only

package gcode

import (
	"fmt"
	"math"
)

// LintRapids reports lateral rapid moves that travel below the program's clearance plane — a G0
// that changes X or Y while the tool sits under the height it retracts to. Such a move drags the
// tool through uncut stock at rapid speed, a classic cause of a broken cutter or a crash, where a
// safe program first retracts to clearance and only then rapids across.
//
// The clearance plane is inferred from the path itself: the highest Z reached by a pure-Z retract
// (a G0 lowering/raising only Z). A lateral rapid before any retract has established that plane is
// left alone — it is the program's initial positioning, made from a safe machine height.
//
// Example: a path that retracts to Z=15, rapids across, then plunges, cuts, and rapids to the next
// hole *without* retracting first yields one warning naming the offending move.
func LintRapids(path Path) []string {
	var warnings []string
	curZ := math.NaN() // current tool Z, unknown until the first Z appears
	clearance := math.Inf(-1)
	for i, c := range path.Commands {
		_, hasX := c.Params["X"]
		_, hasY := c.Params["Y"]
		z, hasZ := c.Params["Z"]
		lateral := hasX || hasY
		if c.Name == "G0" && lateral && !math.IsInf(clearance, -1) && !math.IsNaN(curZ) && curZ < clearance-1e-9 {
			warnings = append(warnings, fmt.Sprintf("move %d: rapid across X/Y at Z=%g is below the clearance plane Z=%g (rapid through stock)", i, curZ, clearance))
		}
		if c.Name == "G0" && hasZ && !lateral { // a pure-Z retract sets/raises the clearance plane
			clearance = math.Max(clearance, z)
		}
		if hasZ {
			curZ = z
		}
	}
	return warnings
}
