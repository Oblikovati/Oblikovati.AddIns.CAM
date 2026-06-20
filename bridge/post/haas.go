// SPDX-License-Identifier: GPL-2.0-only

package post

import (
	"fmt"
	"strconv"
	"strings"

	"oblikovati.org/cam/bridge/gcode"
)

// HaasOptions are the Haas post's knobs. Defaults match a Haas VF-series control: a "%" tape
// wrapper, a five-digit O-number program header, N-sequence numbers, comments on, metric,
// precision 4. Like Fanuc, a Haas keeps canned drill cycles, so they pass through.
type HaasOptions struct {
	OutputComments  bool
	SequenceNumbers bool
	Precision       int
	ProgramNumber   int
	Inches          bool
}

// defaultHaasOptions returns the Haas defaults.
func defaultHaasOptions() HaasOptions {
	return HaasOptions{OutputComments: true, SequenceNumbers: true, Precision: 4, ProgramNumber: 1}
}

// parseHaasArgs applies the argstring onto the defaults.
func parseHaasArgs(argstring string) HaasOptions {
	o := defaultHaasOptions()
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
	return o
}

// haasRenderer carries the Haas export state: the running sequence number.
type haasRenderer struct {
	opts HaasOptions
	seq  int
}

// ExportHaas renders objects to Haas-dialect G-code: a "%" wrapper, a five-digit O-number header,
// a safe-start block that cancels stray modal state (G40 G49 G80), each operation's blocks (canned
// cycles preserved), a G28 Z-home return, and M30.
func ExportHaas(objects []Object, argstring string) string {
	r := &haasRenderer{opts: parseHaasArgs(argstring), seq: 10}
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
func (r *haasRenderer) line(b *strings.Builder, text string) {
	if r.opts.SequenceNumbers {
		b.WriteString("N" + strconv.Itoa(r.seq) + " ")
		r.seq += 10
	}
	b.WriteString(text + "\n")
}

// writeHeader emits the five-digit O-number program line and the Haas safe-start block.
func (r *haasRenderer) writeHeader(b *strings.Builder) {
	prog := fmt.Sprintf("O%05d", r.opts.ProgramNumber)
	if r.opts.OutputComments {
		prog += " (Exported by Oblikovati)"
	}
	r.line(b, prog)
	unit := "G21"
	if r.opts.Inches {
		unit = "G20"
	}
	// Haas safe-start: rapid mode, plane, units, cutter-comp/length-comp/canned-cycle cancel,
	// absolute, and the first work offset.
	r.line(b, "G00 G17 "+unit+" G40 G49 G80 G90 G54")
}

// writeOperation emits an operation's comment and its path blocks.
func (r *haasRenderer) writeOperation(b *strings.Builder, obj Object) {
	if r.opts.OutputComments {
		r.line(b, "("+obj.Label+")")
	}
	for _, c := range obj.Path.Commands {
		if tokens := r.commandTokens(c); len(tokens) > 0 {
			r.line(b, formatOutstring(tokens))
		}
	}
}

// writeFooter stops the spindle, returns Z to home, and ends the program (Haas convention).
func (r *haasRenderer) writeFooter(b *strings.Builder) {
	r.line(b, "M05")
	r.line(b, "G28 G91 Z0.")
	r.line(b, "G90")
	r.line(b, "M30")
}

// commandTokens builds the address tokens for one command (name + parameters in Fanuc's address
// order, which Haas shares). A whole-line comment passes through verbatim.
func (r *haasRenderer) commandTokens(c gcode.Command) []string {
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
func (r *haasRenderer) formatParam(name, p string, v float64) (string, bool) {
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
func (r *haasRenderer) toUnit(mm float64) float64 {
	if r.opts.Inches {
		return mm / 25.4
	}
	return mm
}

// fixed formats with the options' precision.
func (r *haasRenderer) fixed(v float64) string {
	return fmt.Sprintf("%.*f", r.opts.Precision, v)
}
