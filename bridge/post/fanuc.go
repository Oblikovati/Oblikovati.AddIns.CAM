// SPDX-License-Identifier: GPL-2.0-only

package post

import (
	"fmt"
	"strconv"
	"strings"

	"oblikovati.org/cam/bridge/gcode"
)

// FanucOptions are the Fanuc/ISO post's knobs. Defaults match the common industrial dialect:
// a "%" tape wrapper, an O-number program header, N-sequence numbers, comments on, metric,
// precision 3. Fanuc controls keep canned drill cycles (unlike GRBL), so they pass through.
type FanucOptions struct {
	OutputComments  bool
	SequenceNumbers bool
	Precision       int
	ProgramNumber   int
	Inches          bool
}

// defaultFanucOptions returns the Fanuc defaults.
func defaultFanucOptions() FanucOptions {
	return FanucOptions{OutputComments: true, SequenceNumbers: true, Precision: 3, ProgramNumber: 1}
}

// parseFanucArgs applies the argstring onto the defaults.
func parseFanucArgs(argstring string) FanucOptions {
	o := defaultFanucOptions()
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
		}
	}
	if o.Inches {
		o.Precision = 4
	}
	return o
}

// fanucRenderer carries the Fanuc export state: the running sequence number.
type fanucRenderer struct {
	opts FanucOptions
	seq  int
}

// ExportFanuc renders objects to Fanuc/ISO-dialect G-code: a "%" wrapper, an O-number header, a
// metric/absolute preamble, each operation's blocks (canned cycles preserved), and M30.
func ExportFanuc(objects []Object, argstring string) string {
	r := &fanucRenderer{opts: parseFanucArgs(argstring), seq: 10}
	var b strings.Builder
	b.WriteString("%\n")
	r.writeHeader(&b)
	for _, obj := range objects {
		r.writeOperation(&b, obj)
	}
	r.writeFooter(&b)
	b.WriteString("%\n")
	return b.String()
}

// line writes one block with an optional N-sequence prefix, advancing the counter.
func (r *fanucRenderer) line(b *strings.Builder, text string) {
	if r.opts.SequenceNumbers {
		b.WriteString("N" + strconv.Itoa(r.seq) + " ")
		r.seq += 10
	}
	b.WriteString(text + "\n")
}

// writeHeader emits the O-number program line and the metric/absolute/feed-per-minute preamble.
func (r *fanucRenderer) writeHeader(b *strings.Builder) {
	prog := fmt.Sprintf("O%04d", r.opts.ProgramNumber)
	if r.opts.OutputComments {
		prog += " (Exported by Oblikovati)"
	}
	r.line(b, prog)
	unit := "G21"
	if r.opts.Inches {
		unit = "G20"
	}
	r.line(b, "G17 "+unit+" G90 G94 G54")
}

// writeOperation emits an operation's comment and its path blocks.
func (r *fanucRenderer) writeOperation(b *strings.Builder, obj Object) {
	if r.opts.OutputComments {
		r.line(b, "("+obj.Label+")")
	}
	for _, c := range obj.Path.Commands {
		if tokens := r.commandTokens(c); len(tokens) > 0 {
			r.line(b, formatOutstring(tokens))
		}
	}
}

// writeFooter stops the spindle and ends the program.
func (r *fanucRenderer) writeFooter(b *strings.Builder) {
	r.line(b, "M05")
	r.line(b, "M30")
}

// fanucParams is Fanuc's address output order.
var fanucParams = []string{"X", "Y", "Z", "A", "B", "C", "I", "J", "K", "R", "Q", "F", "S", "T", "P", "L"}

// commandTokens builds the address tokens for one command (name + parameters in fanucParams
// order). A whole-line comment (a name starting with "(") passes through verbatim.
func (r *fanucRenderer) commandTokens(c gcode.Command) []string {
	if strings.HasPrefix(c.Name, "(") {
		if r.opts.OutputComments {
			return []string{c.Name}
		}
		return nil
	}
	tokens := []string{c.Name}
	for _, p := range fanucParams {
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
func (r *fanucRenderer) formatParam(name, p string, v float64) (string, bool) {
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
func (r *fanucRenderer) toUnit(mm float64) float64 {
	if r.opts.Inches {
		return mm / 25.4
	}
	return mm
}

// fixed formats with the options' precision.
func (r *fanucRenderer) fixed(v float64) string {
	return fmt.Sprintf("%.*f", r.opts.Precision, v)
}
