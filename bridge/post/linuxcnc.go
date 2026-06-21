// SPDX-License-Identifier: GPL-2.0-only

package post

import (
	"fmt"
	"strconv"
	"strings"

	"oblikovati.org/cam/bridge/gcode"
)

// LinuxCNCOptions are the LinuxCNC post's knobs, parsed from the argstring. Defaults match
// the legacy-dialect globals (comments + header on, line numbers off, precision 3,
// metric, tool-length offsets on).
type LinuxCNCOptions struct {
	OutputHeader      bool
	OutputComments    bool
	OutputLineNumbers bool
	Precision         int
	Inches            bool
	Modal             bool // suppress a repeated command name
	OutputDoubles     bool // when false, suppress an axis value equal to the previous (axis-modal)
	UseTLO            bool // emit G43 Hn after a tool change
	Preamble          string
	Postamble         string
}

// defaultLinuxCNCOptions returns the legacy defaults (PREAMBLE/POSTAMBLE included).
func defaultLinuxCNCOptions() LinuxCNCOptions {
	return LinuxCNCOptions{
		OutputHeader:   true,
		OutputComments: true,
		Precision:      3,
		OutputDoubles:  true,
		UseTLO:         true,
		Preamble:       "G17 G54 G40 G49 G80 G90\n",
		Postamble:      "M05\nG17 G54 G90 G80 G40\nM2\n",
	}
}

// parseLinuxCNCArgs applies the argstring flags onto the defaults (the subset supported
// processArguments. Unknown flags are ignored (e.g. --no-show-editor is a GUI-only no-op
// here). Inches forces precision 4.
func parseLinuxCNCArgs(argstring string) LinuxCNCOptions {
	o := defaultLinuxCNCOptions()
	workOffset := "G54"
	for _, tok := range shlexSplit(argstring) {
		switch {
		case strings.HasPrefix(tok, "--work-offset="):
			workOffset = workOffsetOr(strings.TrimPrefix(tok, "--work-offset="))
		case tok == "--no-header":
			o.OutputHeader = false
		case tok == "--no-comments":
			o.OutputComments = false
		case tok == "--line-numbers":
			o.OutputLineNumbers = true
		case tok == "--modal":
			o.Modal = true
		case tok == "--axis-modal":
			o.OutputDoubles = false
		case tok == "--no-tlo":
			o.UseTLO = false
		case tok == "--inches":
			o.Inches = true
		case strings.HasPrefix(tok, "--precision="):
			if p, err := strconv.Atoi(strings.TrimPrefix(tok, "--precision=")); err == nil {
				o.Precision = p
			}
		case strings.HasPrefix(tok, "--preamble="):
			o.Preamble = strings.ReplaceAll(strings.TrimPrefix(tok, "--preamble="), `\n`, "\n")
		case strings.HasPrefix(tok, "--postamble="):
			o.Postamble = strings.ReplaceAll(strings.TrimPrefix(tok, "--postamble="), `\n`, "\n")
		}
	}
	if o.Inches {
		o.Precision = 4
	}
	if workOffset != "G54" { // swap the WCS into the preamble/postamble; default G54 is left as-is
		o.Preamble = strings.ReplaceAll(o.Preamble, "G54", workOffset)
		o.Postamble = strings.ReplaceAll(o.Postamble, "G54", workOffset)
	}
	return o
}

// unit returns the active length unit code, label, and speed label for the options.
func (o LinuxCNCOptions) unit() (code, format, speed string) {
	if o.Inches {
		return "G20", "in", "in/min"
	}
	return "G21", "mm", "mm/min"
}

// lncRenderer carries the export state: the options and the running line-number counter
// (which spans the whole program, unlike per-path currLocation/lastcommand).
type lncRenderer struct {
	opts   LinuxCNCOptions
	lineNR int
}

// ExportLinuxCNC renders the objects to LinuxCNC-dialect G-code, applying the argstring
// flags. It is validated line-for-line against a reference exact-string oracle.
func ExportLinuxCNC(objects []Object, argstring string) string {
	r := &lncRenderer{opts: parseLinuxCNCArgs(argstring), lineNR: 100}
	var b strings.Builder
	r.writeHeaderAndPreamble(&b)
	for _, obj := range objects {
		r.writeOperation(&b, obj)
	}
	r.writePostamble(&b)
	return b.String()
}

// lineNumber returns the "Nxxx " prefix when line numbers are enabled (incrementing the
// counter), else the empty string.
func (r *lncRenderer) lineNumber() string {
	if !r.opts.OutputLineNumbers {
		return ""
	}
	r.lineNR += 10
	return "N" + strconv.Itoa(r.lineNR) + " "
}

// writeHeaderAndPreamble emits the optional timestamped header, the preamble block, and the
// units line.
func (r *lncRenderer) writeHeaderAndPreamble(b *strings.Builder) {
	if r.opts.OutputHeader {
		b.WriteString(r.lineNumber() + "(Exported by Oblikovati)\n")
		b.WriteString(r.lineNumber() + "(Post Processor: linuxcnc)\n")
		b.WriteString(r.lineNumber() + "(Output Time: generated)\n")
	}
	if r.opts.OutputComments {
		b.WriteString(r.lineNumber() + "(begin preamble)\n")
	}
	for _, line := range strings.Split(strings.TrimRight(r.opts.Preamble, "\n"), "\n") {
		b.WriteString(r.lineNumber() + line + "\n")
	}
	code, _, _ := r.opts.unit()
	b.WriteString(r.lineNumber() + code + "\n")
}

// writeOperation emits one operation's comment framing and its parsed path.
func (r *lncRenderer) writeOperation(b *strings.Builder, obj Object) {
	_, _, speed := r.opts.unit()
	if r.opts.OutputComments {
		b.WriteString(r.lineNumber() + "(begin operation: " + obj.Label + ")\n")
		b.WriteString(r.lineNumber() + "(machine units: " + speed + ")\n")
	}
	b.WriteString(r.parsePath(obj.Path))
	if r.opts.OutputComments {
		b.WriteString(r.lineNumber() + "(finish operation: " + obj.Label + ")\n")
	}
}

// writePostamble emits the (non-line-numbered) begin-postamble comment and the postamble
// block, where "(begin postamble)" carries no line number.
func (r *lncRenderer) writePostamble(b *strings.Builder) {
	if r.opts.OutputComments {
		b.WriteString("(begin postamble)\n")
	}
	for _, line := range strings.Split(strings.TrimRight(r.opts.Postamble, "\n"), "\n") {
		b.WriteString(r.lineNumber() + line + "\n")
	}
}

// lncParams is the LinuxCNC address output order (no K — linuxcnc rejects K on the XY plane).
var lncParams = []string{"X", "Y", "Z", "A", "B", "C", "I", "J", "F", "S", "T", "Q", "R", "L", "H", "D", "P"}

// parsePath renders one path's commands. currLocation/lastcommand are per-path state
// (reset here), seeded with a sentinel first move so the first real move
// always prints its axes under axis-modal.
func (r *lncRenderer) parsePath(path gcode.Path) string {
	var out strings.Builder
	lastCommand := ""
	currLocation := map[string]float64{"X": -1, "Y": -1, "Z": -1, "F": 0}

	for _, c := range path.Commands {
		if strings.HasPrefix(c.Name, "(") && !r.opts.OutputComments {
			continue
		}
		tokens := r.commandTokens(c, lastCommand, currLocation)
		lastCommand = c.Name
		for k, v := range c.Params {
			currLocation[k] = v
		}
		r.applyToolChange(&out, c, &tokens)
		if len(tokens) >= 1 {
			line := strings.Join(tokens, " ") + " \n"
			// The line number (already trailing a space) is inserted as the first
			// token then joins with COMMAND_SPACE, so an enabled line number is followed
			// by two spaces ("N160  G0 ...").
			if r.opts.OutputLineNumbers {
				line = r.lineNumber() + " " + line
			}
			out.WriteString(line)
		}
	}
	return out.String()
}

// commandTokens builds the address tokens for one command (the name, suppressed under
// modal when repeated, followed by its parameters in lncParams order).
func (r *lncRenderer) commandTokens(c gcode.Command, lastCommand string, currLocation map[string]float64) []string {
	tokens := []string{c.Name}
	if r.opts.Modal && c.Name == lastCommand {
		tokens = tokens[:0]
	}
	for _, p := range lncParams {
		v, ok := c.Params[p]
		if !ok {
			continue
		}
		if tok, emit := r.formatParam(c.Name, p, v, currLocation); emit {
			tokens = append(tokens, tok)
		}
	}
	return tokens
}

// formatParam renders one address=value token, returning emit=false when the value should
// be suppressed (a feed on a rapid, a non-positive feed, or an axis equal to the previous
// under axis-modal).
func (r *lncRenderer) formatParam(name, p string, v float64, currLocation map[string]float64) (string, bool) {
	switch p {
	case "F":
		if name == "G0" || name == "G00" {
			return "", false
		}
		if v <= 0 {
			return "", false
		}
		return "F" + r.fixed(v), true
	case "T", "H", "D", "S":
		return p + strconv.Itoa(int(v)), true
	case "A", "B", "C":
		return p + r.fixed(v), true
	default: // X Y Z I J Q R L (lengths)
		if !r.opts.OutputDoubles {
			if prev, ok := currLocation[p]; ok && prev == v {
				return "", false
			}
		}
		return p + r.fixed(r.toUnit(v)), true
	}
}

// applyToolChange emits the spindle stop + appends a tool-length-offset block on M6,
// the M6 handling (M5 line, then G43 Hn appended to the tool-change
// line when TLO is on).
func (r *lncRenderer) applyToolChange(out *strings.Builder, c gcode.Command, tokens *[]string) {
	if c.Name != "M6" {
		return
	}
	out.WriteString(r.lineNumber() + "M5\n")
	if r.opts.UseTLO {
		*tokens = append(*tokens, "\nG43 H"+strconv.Itoa(int(c.Params["T"])))
	}
}

// toUnit converts a millimetre length to the active output unit (identity for mm, /25.4
// for inches).
func (r *lncRenderer) toUnit(mm float64) float64 {
	if r.opts.Inches {
		return mm / 25.4
	}
	return mm
}

// fixed formats a value with the options' fixed decimal precision.
func (r *lncRenderer) fixed(v float64) string {
	return fmt.Sprintf("%.*f", r.opts.Precision, v)
}
