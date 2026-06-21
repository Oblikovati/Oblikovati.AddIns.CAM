// SPDX-License-Identifier: GPL-2.0-only

package post

import (
	"fmt"
	"strconv"
	"strings"

	"oblikovati.org/cam/bridge/gcode"
)

// HeidenhainOptions are the Heidenhain conversational (Klartext) post's knobs. Klartext is not
// ISO G-code: blocks are numbered and motion is written as L / CC / C blocks with signed
// coordinates, FMAX rapids, and TOOL CALL tool changes. This post emits the milling core and
// expands canned drill cycles to explicit moves (Klartext fixed cycles — CYCL DEF — are a
// follow-up). Targets TNC 320/620-class controls.
type HeidenhainOptions struct {
	OutputComments bool
	Precision      int
	Inches         bool
	ProgramName    string
}

// parseHeidenhainArgs reads the shared scalar flags plus the Klartext-specific --program= name.
func parseHeidenhainArgs(argstring string) HeidenhainOptions {
	comments, inches, precision := parseScalarPostArgs(argstring, 3)
	o := HeidenhainOptions{OutputComments: comments, Inches: inches, Precision: precision, ProgramName: "OBLIKOVATI"}
	for _, tok := range shlexSplit(argstring) {
		if strings.HasPrefix(tok, "--program=") {
			o.ProgramName = strings.TrimPrefix(tok, "--program=")
		}
	}
	return o
}

// heidenhainRenderer carries the Klartext export state: the running block number, the current
// tool position (for arc centres), and the tool number pending from a tool-change command.
type heidenhainRenderer struct {
	opts             HeidenhainOptions
	block            int
	curX, curY, curZ float64
	pendingTool      int
}

// ExportHeidenhain renders objects to Heidenhain Klartext: a numbered BEGIN PGM … END PGM program
// with TOOL CALL changes, L linear moves (FMAX rapids), CC/C circular moves, and comments.
func ExportHeidenhain(objects []Object, argstring string) string {
	r := &heidenhainRenderer{opts: parseHeidenhainArgs(argstring)}
	var b strings.Builder
	r.line(&b, "BEGIN PGM "+r.opts.ProgramName+" "+r.unit())
	for _, obj := range objects {
		r.comment(&b, obj.Label)
		for _, c := range obj.Path.Commands {
			r.writeCommand(&b, c)
			r.track(c)
		}
	}
	r.line(&b, "END PGM "+r.opts.ProgramName+" "+r.unit())
	return b.String()
}

// unit returns the Klartext program unit word.
func (r *heidenhainRenderer) unit() string {
	if r.opts.Inches {
		return "INCH"
	}
	return "MM"
}

// line writes one numbered Klartext block and advances the block counter.
func (r *heidenhainRenderer) line(b *strings.Builder, text string) {
	b.WriteString(strconv.Itoa(r.block) + " " + text + "\n")
	r.block++
}

// comment writes a numbered ";text" comment block when comments are enabled.
func (r *heidenhainRenderer) comment(b *strings.Builder, text string) {
	if r.opts.OutputComments {
		r.line(b, "; "+text)
	}
}

// writeCommand emits the Klartext block(s) for one G-code command. ISO modal/plane codes
// (G17/G40/G54/G80/G90/G94/G98/G99) have no Klartext equivalent and are dropped.
func (r *heidenhainRenderer) writeCommand(b *strings.Builder, c gcode.Command) {
	switch c.Name {
	case "G0", "G00":
		r.move(b, c, true)
	case "G1", "G01":
		r.move(b, c, false)
	case "G2", "G02":
		r.arc(b, c, true)
	case "G3", "G03":
		r.arc(b, c, false)
	case "G81", "G82", "G83", "G84", "G74", "G85":
		r.drill(b, c)
	case "M6", "M06":
		if t, ok := c.Params["T"]; ok {
			r.pendingTool = int(t)
		}
	case "M3", "M03":
		r.toolCall(b, c, false)
	case "M4", "M04":
		r.toolCall(b, c, true)
	case "M5", "M05":
		r.line(b, "M5")
	case "M0", "M00":
		r.line(b, "M0")
	case "M1", "M01":
		r.line(b, "M1")
	default:
		if strings.HasPrefix(c.Name, "(") {
			r.comment(b, strings.Trim(c.Name, "()"))
		}
	}
}

// move emits an L block for a linear move: the present axes as signed coordinates, no radius
// compensation (R0), and either FMAX for a rapid or the feed for a cut (omitted when modal).
func (r *heidenhainRenderer) move(b *strings.Builder, c gcode.Command, rapid bool) {
	parts := append([]string{"L"}, r.axisWords(c)...)
	parts = append(parts, "R0")
	if rapid {
		parts = append(parts, "FMAX")
	} else if f, ok := c.Params["F"]; ok && f > 0 {
		parts = append(parts, "F"+strconv.Itoa(int(f)))
	}
	r.line(b, strings.Join(parts, " "))
}

// arc emits a CC (circle centre, from the I/J offsets relative to the current point) then a C
// block to the endpoint with the rotation sense (DR- clockwise / DR+ counter-clockwise).
func (r *heidenhainRenderer) arc(b *strings.Builder, c gcode.Command, clockwise bool) {
	cx, cy := r.curX+c.Params["I"], r.curY+c.Params["J"]
	r.line(b, "CC X"+r.coord(cx)+" Y"+r.coord(cy))
	parts := append([]string{"C"}, r.axisWords(c)...)
	if clockwise {
		parts = append(parts, "DR-")
	} else {
		parts = append(parts, "DR+")
	}
	if f, ok := c.Params["F"]; ok && f > 0 {
		parts = append(parts, "F"+strconv.Itoa(int(f)))
	}
	r.line(b, strings.Join(parts, " "))
}

// axisWords formats the present X/Y/Z of a command as signed Klartext coordinate words.
func (r *heidenhainRenderer) axisWords(c gcode.Command) []string {
	var words []string
	for _, ax := range []string{"X", "Y", "Z"} {
		if v, ok := c.Params[ax]; ok {
			words = append(words, ax+r.coord(v))
		}
	}
	return words
}

// toolCall emits a TOOL CALL block (tool number, spindle axis Z, spindle speed) and the spindle
// direction M-function. The tool number was captured from the preceding M6.
func (r *heidenhainRenderer) toolCall(b *strings.Builder, c gcode.Command, reverse bool) {
	call := fmt.Sprintf("TOOL CALL %d Z", r.pendingTool)
	if s, ok := c.Params["S"]; ok {
		call += " S" + strconv.Itoa(int(s))
	}
	r.line(b, call)
	if reverse {
		r.line(b, "M4")
	} else {
		r.line(b, "M3")
	}
}

// drill expands a canned drilling/tapping cycle into explicit L moves (Klartext has no canned
// cycle here): rapid over the hole and to the R plane, feed to depth (pecking for G83, feeding
// back out for the tapping/boring cycles), then rapid clear. A comment marks the expansion.
func (r *heidenhainRenderer) drill(b *strings.Builder, c gcode.Command) {
	x, y, z, rp := c.Params["X"], c.Params["Y"], c.Params["Z"], c.Params["R"]
	if rp < z {
		r.comment(b, "drill cycle error: R below Z")
		return
	}
	r.comment(b, c.Name+" cycle expanded to moves")
	feed := strconv.Itoa(int(c.Params["F"]))
	r.line(b, "L X"+r.coord(x)+" Y"+r.coord(y)+" R0 FMAX")
	r.line(b, "L Z"+r.coord(rp)+" R0 FMAX")
	if c.Name == "G83" && c.Params["Q"] > 0 {
		r.peck(b, z, rp, c.Params["Q"], feed)
	} else {
		r.line(b, "L Z"+r.coord(z)+" R0 F"+feed)
	}
	r.retractFromDrill(b, c, rp, feed)
}

// retractFromDrill leaves the hole: the tapping (G84/G74) and boring (G85) cycles feed back out at
// the cutting feed, the plain/peck/dwell cycles rapid out.
func (r *heidenhainRenderer) retractFromDrill(b *strings.Builder, c gcode.Command, rp float64, feed string) {
	switch c.Name {
	case "G84", "G74", "G85":
		r.line(b, "L Z"+r.coord(rp)+" R0 F"+feed)
	default:
		r.line(b, "L Z"+r.coord(rp)+" R0 FMAX")
	}
}

// peck feeds to depth in Q increments, rapiding back to the R plane between pecks to clear chips.
func (r *heidenhainRenderer) peck(b *strings.Builder, z, rp, step float64, feed string) {
	depth := rp
	for depth > z {
		depth -= step
		if depth < z {
			depth = z
		}
		r.line(b, "L Z"+r.coord(depth)+" R0 F"+feed)
		if depth > z {
			r.line(b, "L Z"+r.coord(rp)+" R0 FMAX")
		}
	}
}

// coord formats a length as a signed Klartext coordinate at the configured precision (inches
// convert from the millimetre model).
func (r *heidenhainRenderer) coord(v float64) string {
	if r.opts.Inches {
		v /= 25.4
	}
	return fmt.Sprintf("%+.*f", r.opts.Precision, v)
}

// track updates the current tool position from a command's X/Y/Z so the next arc's centre is
// computed from the right point.
func (r *heidenhainRenderer) track(c gcode.Command) {
	if v, ok := c.Params["X"]; ok {
		r.curX = v
	}
	if v, ok := c.Params["Y"]; ok {
		r.curY = v
	}
	if v, ok := c.Params["Z"]; ok {
		r.curZ = v
	}
}
