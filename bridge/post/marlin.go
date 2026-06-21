// SPDX-License-Identifier: GPL-2.0-only

package post

import (
	"fmt"
	"strconv"
	"strings"

	"oblikovati.org/cam/bridge/gcode"
)

// MarlinOptions are the Marlin post's knobs. Defaults suit a hobby CNC / diode laser running
// Marlin or RepRap firmware: semicolon comments, metric, precision 3, canned drill cycles
// translated to G0/G1 (Marlin has none), tool changes turned into an M0 pause for a manual swap.
type MarlinOptions struct {
	OutputComments bool
	Precision      int
	Inches         bool
}

// parseMarlinArgs reads the shared scalar flags (comments / inches / precision) into the options.
func parseMarlinArgs(argstring string) MarlinOptions {
	comments, inches, precision := parseScalarPostArgs(argstring, 3)
	return MarlinOptions{OutputComments: comments, Inches: inches, Precision: precision}
}

// marlinRenderer carries the Marlin export state: the current tool position (for translating
// canned cycles to explicit moves).
type marlinRenderer struct {
	opts             MarlinOptions
	curX, curY, curZ float64
}

// ExportMarlin renders objects to Marlin/RepRap-dialect G-code: semicolon comments, a metric
// absolute preamble, each operation's blocks (canned drill cycles expanded, tool changes paused),
// and a spindle-off footer.
func ExportMarlin(objects []Object, argstring string) string {
	r := &marlinRenderer{opts: parseMarlinArgs(argstring)}
	var b strings.Builder
	r.comment(&b, "Exported by Oblikovati")
	r.comment(&b, "Post Processor: marlin")
	unit := "G21"
	if r.opts.Inches {
		unit = "G20"
	}
	b.WriteString(unit + "\n")
	b.WriteString("G90\n")
	for _, obj := range objects {
		r.comment(&b, obj.Label)
		r.writePath(&b, obj)
	}
	b.WriteString("M5\n")
	r.comment(&b, "End")
	return b.String()
}

// comment writes a ";text" line when comments are enabled.
func (r *marlinRenderer) comment(b *strings.Builder, text string) {
	if r.opts.OutputComments {
		b.WriteString("; " + text + "\n")
	}
}

// writePath renders one object's commands, translating canned drill cycles and tool changes.
func (r *marlinRenderer) writePath(b *strings.Builder, obj Object) {
	for _, c := range obj.Path.Commands {
		switch {
		case c.Name == "G81" || c.Name == "G82" || c.Name == "G83":
			r.drillTranslate(b, c)
		case c.Name == "G84" || c.Name == "G74":
			r.tapTranslate(b, c)
		case c.Name == "M6" || c.Name == "M06":
			r.comment(b, "tool change — pausing for a manual swap")
			b.WriteString("M0\n")
		case strings.HasPrefix(c.Name, "("):
			r.comment(b, strings.Trim(c.Name, "()"))
		default:
			if line := r.commandLine(c); line != "" {
				b.WriteString(line + "\n")
			}
		}
		r.trackPosition(c)
	}
}

// marlinParams is Marlin's address output order.
var marlinParams = []string{"X", "Y", "Z", "I", "J", "K", "F", "S", "T"}

// commandLine formats one ordinary command (name + parameters in marlinParams order), or "" for a
// bare/empty command.
func (r *marlinRenderer) commandLine(c gcode.Command) string {
	tokens := []string{c.Name}
	for _, p := range marlinParams {
		if v, ok := c.Params[p]; ok {
			if tok, emit := r.formatParam(c.Name, p, v); emit {
				tokens = append(tokens, tok)
			}
		}
	}
	if len(tokens) == 1 && tokens[0] == "" {
		return ""
	}
	return formatOutstring(tokens)
}

// formatParam formats one address value: feeds suppressed on rapids, spindle/tool as integers,
// the rest as unit-converted fixed-precision lengths.
func (r *marlinRenderer) formatParam(name, p string, v float64) (string, bool) {
	switch p {
	case "F":
		if name == "G0" || name == "G00" || v <= 0 {
			return "", false
		}
		return "F" + r.fixed(v), true
	case "S", "T":
		return p + strconv.Itoa(int(v)), true
	default: // X Y Z I J K (lengths)
		return p + r.fixed(r.toUnit(v)), true
	}
}

// drillTranslate expands a G81/G82/G83 canned cycle into explicit moves: rapid over the hole and
// to the retract plane, plunge to depth (pecking for G83, dwelling for G82), and retract.
func (r *marlinRenderer) drillTranslate(b *strings.Builder, c gcode.Command) {
	x, y := r.coord(c, "X", r.curX), r.coord(c, "Y", r.curY)
	z, rPlane := c.Params["Z"], c.Params["R"]
	if rPlane < z {
		r.comment(b, "drill cycle error: R below Z")
		return
	}
	feed := r.feedSuffix(c.Params["F"])
	b.WriteString("G0 X" + r.fixed(r.toUnit(x)) + " Y" + r.fixed(r.toUnit(y)) + "\n")
	b.WriteString("G0 Z" + r.fixed(r.toUnit(rPlane)) + "\n")
	switch c.Name {
	case "G83":
		r.peck(b, z, rPlane, c.Params["Q"], feed)
	default: // G81 / G82
		b.WriteString("G1 Z" + r.fixed(r.toUnit(z)) + feed)
		if c.Name == "G82" {
			b.WriteString("G4 P" + strconv.FormatFloat(c.Params["P"], 'g', -1, 64) + "\n")
		}
	}
	b.WriteString("G0 Z" + r.fixed(r.toUnit(rPlane)) + "\n")
}

// tapTranslate expands a tapping canned cycle (G84 right-hand / G74 left-hand) into explicit
// moves for firmware without rigid tapping: rapid over the hole and to the retract plane, feed
// down to depth at the synchronised feed, then feed back out at the same feed. This is the
// motion a self-reversing (tension-compression) tapping head needs; the head handles the spindle
// reversal, so a comment flags the soft tap.
func (r *marlinRenderer) tapTranslate(b *strings.Builder, c gcode.Command) {
	x, y := r.coord(c, "X", r.curX), r.coord(c, "Y", r.curY)
	z, rPlane := c.Params["Z"], c.Params["R"]
	if rPlane < z {
		r.comment(b, "tap cycle error: R below Z")
		return
	}
	hand := "right-hand"
	if c.Name == "G74" {
		hand = "left-hand"
	}
	r.comment(b, "soft tap "+hand+": feed in/out, spindle reverse by tapping head")
	feed := r.feedSuffix(c.Params["F"])
	b.WriteString("G0 X" + r.fixed(r.toUnit(x)) + " Y" + r.fixed(r.toUnit(y)) + "\n")
	b.WriteString("G0 Z" + r.fixed(r.toUnit(rPlane)) + "\n")
	b.WriteString("G1 Z" + r.fixed(r.toUnit(z)) + feed)
	b.WriteString("G1 Z" + r.fixed(r.toUnit(rPlane)) + feed)
}

// peck emits the G83 peck moves: descend in Q-step increments, retracting to the R plane after
// each peck, until the final depth is reached. A zero/negative step plunges straight to depth.
func (r *marlinRenderer) peck(b *strings.Builder, z, rPlane, step float64, feed string) {
	if step <= 0 {
		b.WriteString("G1 Z" + r.fixed(r.toUnit(z)) + feed)
		return
	}
	depth := rPlane - step
	for depth > z {
		b.WriteString("G1 Z" + r.fixed(r.toUnit(depth)) + feed)
		b.WriteString("G0 Z" + r.fixed(r.toUnit(rPlane)) + "\n")
		depth -= step
	}
	b.WriteString("G1 Z" + r.fixed(r.toUnit(z)) + feed)
}

// coord returns a command's address value, or the fallback when absent.
func (r *marlinRenderer) coord(c gcode.Command, addr string, fallback float64) float64 {
	if v, ok := c.Params[addr]; ok {
		return v
	}
	return fallback
}

// trackPosition updates the current tool position from a motion command.
func (r *marlinRenderer) trackPosition(c gcode.Command) {
	if x, ok := c.Params["X"]; ok {
		r.curX = x
	}
	if y, ok := c.Params["Y"]; ok {
		r.curY = y
	}
	if z, ok := c.Params["Z"]; ok {
		r.curZ = z
	}
}

// toUnit converts mm to the output unit.
func (r *marlinRenderer) toUnit(mm float64) float64 {
	if r.opts.Inches {
		return mm / 25.4
	}
	return mm
}

// fixed formats with the options' precision.
func (r *marlinRenderer) fixed(v float64) string {
	return fmt.Sprintf("%.*f", r.opts.Precision, v)
}

// feedSuffix renders the plunge-feed suffix (" F..\n") in the active speed unit.
func (r *marlinRenderer) feedSuffix(f float64) string {
	if r.opts.Inches {
		f /= 25.4
	}
	return " F" + r.fixed(f) + "\n"
}
