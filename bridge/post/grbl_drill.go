// SPDX-License-Identifier: GPL-2.0-only

package post

import (
	"fmt"
	"strconv"
	"strings"

	"oblikovati.org/cam/bridge/gcode"
)

// drillTranslate expands a canned drilling cycle (G81/G82/G83) into the explicit G0/G1
// moves GRBL understands, since GRBL has no canned cycles. It follows the NIST-RS274 motion:
// position over
// the hole at the R plane, plunge at feed, retract; for G83 peck in Q increments with a
// small rapid back to just above the previous depth between pecks. All emitted lines carry
// the running line number.
func (r *grblRenderer) drillTranslate(tokens []string, c gcode.Command) string {
	var b strings.Builder
	if r.opts.OutputComments && len(tokens) > 0 {
		commented := append([]string(nil), tokens...)
		commented[0] = "(" + commented[0]
		commented[len(commented)-1] += ")"
		b.WriteString(r.lineNumber() + formatOutstring(commented) + "\n")
	}

	x, y, z, rPlane := c.Params["X"], c.Params["Y"], c.Params["Z"], c.Params["R"]
	if rPlane < z {
		b.WriteString(r.lineNumber() + "(drill cycle error: R less than Z )\n")
		return b.String()
	}

	clearZ := r.clearanceZ(rPlane)
	feed := r.drillFeed(c.Params["F"])

	if r.curZ < rPlane {
		b.WriteString(r.lineNumber() + "G0 Z" + r.fixed(r.toUnit(rPlane)) + "\n")
	}
	b.WriteString(r.lineNumber() + "G0 X" + r.fixed(r.toUnit(x)) + " Y" + r.fixed(r.toUnit(y)) + "\n")
	if r.curZ > rPlane {
		b.WriteString(r.lineNumber() + "G0 Z" + r.fixed(r.toUnit(rPlane)) + "\n")
	}

	switch c.Name {
	case "G81", "G82":
		b.WriteString(r.lineNumber() + "G1 Z" + r.fixed(r.toUnit(z)) + feed)
		if c.Name == "G82" {
			b.WriteString(r.lineNumber() + "G4 P" + strconv.FormatFloat(c.Params["P"], 'g', -1, 64) + "\n")
		}
		b.WriteString(r.lineNumber() + "G0 Z" + r.fixed(r.toUnit(clearZ)) + "\n")
	case "G83":
		r.peck(&b, z, rPlane, clearZ, c.Params["Q"], feed)
	}
	return b.String()
}

// tapTranslate expands a tapping canned cycle (G84 right-hand / G74 left-hand) into the
// explicit moves a controller without rigid tapping understands: position over the hole at the
// R plane, feed down to depth at the synchronised feed, then feed back out at the same feed.
// This is the motion a tension-compression (self-reversing) tapping head needs; the spindle
// direction reversal is the head's job, so a comment flags the soft tap. GRBL has no canned
// cycles, so this is the only way the operation survives to the controller.
func (r *grblRenderer) tapTranslate(tokens []string, c gcode.Command) string {
	var b strings.Builder
	if r.opts.OutputComments && len(tokens) > 0 {
		commented := append([]string(nil), tokens...)
		commented[0] = "(" + commented[0]
		commented[len(commented)-1] += ")"
		b.WriteString(r.lineNumber() + formatOutstring(commented) + "\n")
	}
	hand := "right-hand"
	if c.Name == "G74" {
		hand = "left-hand"
	}
	if r.opts.OutputComments {
		b.WriteString(r.lineNumber() + "(soft tap " + hand + ": feed in/out, spindle reverse by tapping head )\n")
	}

	x, y, z, rPlane := c.Params["X"], c.Params["Y"], c.Params["Z"], c.Params["R"]
	if rPlane < z {
		b.WriteString(r.lineNumber() + "(tap cycle error: R less than Z )\n")
		return b.String()
	}
	feed := r.drillFeed(c.Params["F"])
	if r.curZ < rPlane {
		b.WriteString(r.lineNumber() + "G0 Z" + r.fixed(r.toUnit(rPlane)) + "\n")
	}
	b.WriteString(r.lineNumber() + "G0 X" + r.fixed(r.toUnit(x)) + " Y" + r.fixed(r.toUnit(y)) + "\n")
	if r.curZ > rPlane {
		b.WriteString(r.lineNumber() + "G0 Z" + r.fixed(r.toUnit(rPlane)) + "\n")
	}
	b.WriteString(r.lineNumber() + "G1 Z" + r.fixed(r.toUnit(z)) + feed)
	b.WriteString(r.lineNumber() + "G1 Z" + r.fixed(r.toUnit(rPlane)) + feed)
	return b.String()
}

// peck emits the G83 peck-drilling moves: descend in Q-step increments, retracting to the
// clearance plane after each peck and rapiding back to just above the last depth, until the
// final depth is reached (each retract clears by 5% of the step). A zero step emits nothing
// (an infinite-loop guard).
func (r *grblRenderer) peck(b *strings.Builder, z, rPlane, clearZ, step float64, feed string) {
	if step == 0 {
		return
	}
	aBit := step * 0.05
	lastStopZ := rPlane
	for {
		if lastStopZ != clearZ {
			b.WriteString(r.lineNumber() + "G0 Z" + r.fixed(r.toUnit(lastStopZ+aBit)) + "\n")
		}
		nextStopZ := lastStopZ - step
		if nextStopZ > z {
			b.WriteString(r.lineNumber() + "G1 Z" + r.fixed(r.toUnit(nextStopZ)) + feed)
			b.WriteString(r.lineNumber() + "G0 Z" + r.fixed(r.toUnit(clearZ)) + "\n")
			lastStopZ = nextStopZ
			continue
		}
		b.WriteString(r.lineNumber() + "G1 Z" + r.fixed(r.toUnit(z)) + feed)
		b.WriteString(r.lineNumber() + "G0 Z" + r.fixed(r.toUnit(clearZ)) + "\n")
		return
	}
}

// clearanceZ computes the cycle's retract level from the active G98/G99 mode and the
// current tool Z.
func (r *grblRenderer) clearanceZ(rPlane float64) float64 {
	if r.drillRetract == "G98" && r.curZ >= rPlane {
		return r.curZ
	}
	return rPlane
}

// drillFeed renders the plunge feed suffix (" F..\n") at 2 decimals in the active speed
// unit (mm/min, or in/min under inches).
func (r *grblRenderer) drillFeed(f float64) string {
	if r.opts.Inches {
		f /= 25.4
	}
	return fmt.Sprintf(" F%.2f\n", f)
}
