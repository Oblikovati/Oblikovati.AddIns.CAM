// SPDX-License-Identifier: GPL-2.0-only

package link

import (
	"errors"
	"math"
	"strings"

	"oblikovati.org/cam/bridge/gcode"
)

// OptimizeRetracts rewrites a toolpath so that every full retract between two cuts becomes a
// keep-tool-down link when the straight move clears the part. It finds each maximal run of rapid
// (G0) moves that sits *between* two cutting moves — the lift→traverse→plunge the generators emit —
// and replans it with GetLinkingMoves: the tool stays down at the shallower cut depth if that travel
// is clear, else it lifts only to the lowest clearing plane (safeZ, then clearanceZ). Leading and
// trailing rapids (the approach and the final retract) and canned cycles are left untouched. When no
// retract height clears the part the original (always-safe) retract is kept. Heights are mm.
func OptimizeRetracts(path gcode.Path, safeZ, clearanceZ float64, probe CollisionProbe, toolRadius, clearance float64) (gcode.Path, error) {
	cmds := path.Commands
	out := make([]gcode.Command, 0, len(cmds))
	pos := gcode.Vector3{}
	lastMotionWasCut := false
	for i := 0; i < len(cmds); {
		c := cmds[i]
		if !isRapid(c) {
			out = append(out, c)
			if isCut(c) {
				lastMotionWasCut = true
			}
			pos = applyMove(pos, c)
			i++
			continue
		}
		before, end, j := scanRapidRun(cmds, i, pos)
		if lastMotionWasCut && nextMotionIsCut(cmds, j) {
			linked, err := relinkRun(before, end, cmds[i:j], safeZ, clearanceZ, probe, toolRadius, clearance)
			if err != nil {
				return path, err
			}
			out = append(out, linked...)
		} else {
			out = append(out, cmds[i:j]...) // leading/trailing rapid — not a between-cut link
		}
		lastMotionWasCut = false
		pos, i = end, j
	}
	return gcode.NewPath(out), nil
}

// relinkRun replans one between-cut rapid run, falling back to the original retract when no height
// clears the part (always safe). A non-link error from the probe surfaces.
func relinkRun(before, end gcode.Vector3, original []gcode.Command, safeZ, clearanceZ float64, probe CollisionProbe, toolRadius, clearance float64) ([]gcode.Command, error) {
	heights := []float64{math.Max(before.Z, end.Z), safeZ, clearanceZ}
	moves, err := GetLinkingMoves(before, end, heights, probe, toolRadius, clearance)
	switch {
	case err == nil:
		return moves, nil
	case errors.Is(err, ErrNoClearLink):
		return original, nil
	default:
		return nil, err
	}
}

// scanRapidRun consumes the maximal run of consecutive rapid moves starting at index i, returning
// the position before the run, the position after it, and the index just past it.
func scanRapidRun(cmds []gcode.Command, i int, pos gcode.Vector3) (before, end gcode.Vector3, next int) {
	before, end = pos, pos
	j := i
	for j < len(cmds) && isRapid(cmds[j]) {
		end = applyMove(end, cmds[j])
		j++
	}
	return before, end, j
}

// nextMotionIsCut reports whether the next motion command at or after index j is a cut, skipping
// comments and non-motion words (M-codes, spindle, etc.) — so a comment between the retract and the
// next cut does not hide the link.
func nextMotionIsCut(cmds []gcode.Command, j int) bool {
	for ; j < len(cmds); j++ {
		if isCut(cmds[j]) {
			return true
		}
		if isRapid(cmds[j]) {
			return false
		}
	}
	return false
}

// applyMove advances a position by a command's present X/Y/Z params (G-code is modal: an absent
// axis keeps its previous value).
func applyMove(pos gcode.Vector3, c gcode.Command) gcode.Vector3 {
	if v, ok := c.Params["X"]; ok {
		pos.X = v
	}
	if v, ok := c.Params["Y"]; ok {
		pos.Y = v
	}
	if v, ok := c.Params["Z"]; ok {
		pos.Z = v
	}
	return pos
}

// isRapid reports whether a command is a rapid traverse (G0/G00).
func isRapid(c gcode.Command) bool {
	n := strings.ToUpper(strings.TrimSpace(c.Name))
	return n == "G0" || n == "G00"
}

// isCut reports whether a command is a feed-rate cutting move (linear G1 or circular G2/G3); canned
// drill cycles (G81+) handle their own retraction and are not treated as cuts here.
func isCut(c gcode.Command) bool {
	switch strings.ToUpper(strings.TrimSpace(c.Name)) {
	case "G1", "G01", "G2", "G02", "G3", "G03":
		return true
	default:
		return false
	}
}
