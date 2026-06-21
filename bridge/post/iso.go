// SPDX-License-Identifier: GPL-2.0-only

package post

import (
	"fmt"
	"strconv"
	"strings"

	"oblikovati.org/cam/bridge/gcode"
)

// isoOptions are the knobs shared by the O-number ISO posts (Fanuc, Haas): a "%" tape wrapper,
// an O-number header, N-sequence numbers, comments, units, and numeric precision. Each post
// supplies its own program-header and footer blocks around this common body.
type isoOptions struct {
	OutputComments  bool
	SequenceNumbers bool
	Precision       int
	ProgramNumber   int
	Inches          bool
	WorkOffset      string // active work coordinate system (G54..G59); empty → G54
	UseTLO          bool   // emit G43 H<n> (tool-length offset) after each tool change
}

// workOffsetOr returns a valid G54..G59 work-offset word, defaulting blanks/garbage to G54.
func workOffsetOr(s string) string {
	switch s {
	case "G54", "G55", "G56", "G57", "G58", "G59":
		return s
	default:
		return "G54"
	}
}

// parseISOArgs applies the common ISO flags onto the given defaults.
func parseISOArgs(argstring string, o isoOptions) isoOptions {
	for _, tok := range shlexSplit(argstring) {
		switch {
		case tok == "--no-comments":
			o.OutputComments = false
		case tok == "--no-sequence-numbers":
			o.SequenceNumbers = false
		case tok == "--inches":
			o.Inches = true
		case strings.HasPrefix(tok, "--precision="):
			if p, err := strconv.Atoi(strings.TrimPrefix(tok, "--precision=")); err == nil {
				o.Precision = p
			}
		case strings.HasPrefix(tok, "--program-number="):
			if n, err := strconv.Atoi(strings.TrimPrefix(tok, "--program-number=")); err == nil {
				o.ProgramNumber = n
			}
		case strings.HasPrefix(tok, "--work-offset="):
			o.WorkOffset = workOffsetOr(strings.TrimPrefix(tok, "--work-offset="))
		case tok == "--no-tlo":
			o.UseTLO = false
		}
	}
	return o
}

// isoRenderer carries the shared ISO export state (options + running sequence number) and the
// common formatting/body methods. The Fanuc and Haas posts wrap it with their own header/footer.
type isoRenderer struct {
	opts isoOptions
	seq  int
}

// isoParams is the ISO address output order (shared by Fanuc and Haas).
var isoParams = []string{"X", "Y", "Z", "A", "B", "C", "I", "J", "K", "R", "Q", "F", "S", "T", "P", "L"}

// line writes one block with an optional N-sequence prefix, advancing the counter.
func (r *isoRenderer) line(b *strings.Builder, text string) {
	if r.opts.SequenceNumbers {
		b.WriteString("N" + strconv.Itoa(r.seq) + " ")
		r.seq += 10
	}
	b.WriteString(text + "\n")
}

// programLine builds the "O<number> (comment)" header line for the given O-number width.
func (r *isoRenderer) programLine(width int) string {
	prog := fmt.Sprintf("O%0*d", width, r.opts.ProgramNumber)
	if r.opts.OutputComments {
		prog += " (Exported by Oblikovati)"
	}
	return prog
}

// unitCode returns the active length-unit G-code.
func (r *isoRenderer) unitCode() string {
	if r.opts.Inches {
		return "G20"
	}
	return "G21"
}

// writeOperation emits an operation's comment and its path blocks.
func (r *isoRenderer) writeOperation(b *strings.Builder, obj Object) {
	if r.opts.OutputComments {
		r.line(b, "("+obj.Label+")")
	}
	for _, c := range obj.Path.Commands {
		if c.Name == "M6" || c.Name == "M06" {
			r.line(b, "M5") // stop the spindle before the tool change
		}
		if tokens := r.commandTokens(c); len(tokens) > 0 {
			r.line(b, formatOutstring(tokens))
		}
		r.toolLengthOffset(b, c)
	}
}

// toolLengthOffset emits a G43 H<n> tool-length-offset activation right after a tool change
// (M6 T<n>), as Fanuc/Haas controls require before the first Z move on the new tool. The offset
// register defaults to the tool number. Skipped when UseTLO is off.
func (r *isoRenderer) toolLengthOffset(b *strings.Builder, c gcode.Command) {
	if !r.opts.UseTLO || (c.Name != "M6" && c.Name != "M06") {
		return
	}
	if t, ok := c.Params["T"]; ok {
		r.line(b, "G43 H"+strconv.Itoa(int(t)))
	}
}

// commandTokens builds the address tokens for one command (name + parameters in isoParams order).
// A whole-line comment passes through verbatim (or is dropped when comments are off).
func (r *isoRenderer) commandTokens(c gcode.Command) []string {
	if strings.HasPrefix(c.Name, "(") {
		if r.opts.OutputComments {
			return []string{c.Name}
		}
		return nil
	}
	tokens := []string{c.Name}
	for _, p := range isoParams {
		if v, ok := c.Params[p]; ok {
			if tok, emit := r.formatParam(c.Name, p, v); emit {
				tokens = append(tokens, tok)
			}
		}
	}
	return tokens
}

// formatParam formats one address value: feeds suppressed on rapids, spindle/tool/loop counts as
// integers, the rest as unit-converted fixed-precision lengths.
func (r *isoRenderer) formatParam(name, p string, v float64) (string, bool) {
	switch p {
	case "F":
		if name == "G0" || name == "G00" || v <= 0 {
			return "", false
		}
		return "F" + r.fixed(v), true
	case "S", "T", "P", "L":
		return p + strconv.Itoa(int(v)), true
	case "A", "B", "C":
		return p + r.fixed(v), true
	default: // X Y Z I J K R Q (lengths)
		return p + r.fixed(r.toUnit(v)), true
	}
}

// toUnit converts mm to the output unit.
func (r *isoRenderer) toUnit(mm float64) float64 {
	if r.opts.Inches {
		return mm / 25.4
	}
	return mm
}

// fixed formats with the options' precision.
func (r *isoRenderer) fixed(v float64) string {
	return fmt.Sprintf("%.*f", r.opts.Precision, v)
}
