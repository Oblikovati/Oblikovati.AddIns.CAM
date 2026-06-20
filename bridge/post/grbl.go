// SPDX-License-Identifier: GPL-2.0-only

package post

import (
	"fmt"
	"strconv"
	"strings"

	"oblikovati.org/cam/bridge/gcode"
)

// GRBLOptions are the GRBL post's knobs. Defaults match the upstream legacy post: comments
// + header on, metric, precision 3, drill cycles translated to G0/G1 (GRBL has no canned
// cycles), tool changes commented out (GRBL ignores M6).
type GRBLOptions struct {
	OutputHeader     bool
	OutputComments   bool
	OutputLineNumber bool
	Precision        int
	Inches           bool
	Modal            bool
	TranslateDrill   bool
	OutputToolChange bool
	Preamble         string
	Postamble        string
}

// defaultGRBLOptions returns the upstream GRBL defaults.
func defaultGRBLOptions() GRBLOptions {
	return GRBLOptions{
		OutputHeader:   true,
		OutputComments: true,
		Precision:      3,
		TranslateDrill: true,
		Preamble:       "G17 G90\n",
		Postamble:      "M5\nG17 G90\nM2\n",
	}
}

// parseGRBLArgs applies the argstring onto the defaults (the flag subset the GRBL post and
// its oracle exercise). Inches forces precision 4.
func parseGRBLArgs(argstring string) GRBLOptions {
	o := defaultGRBLOptions()
	for _, tok := range shlexSplit(argstring) {
		switch {
		case tok == "--no-header":
			o.OutputHeader = false
		case tok == "--no-comments":
			o.OutputComments = false
		case tok == "--line-numbers":
			o.OutputLineNumber = true
		case tok == "--modal":
			o.Modal = true
		case tok == "--inches":
			o.Inches = true
		case tok == "--no-translate_drill":
			o.TranslateDrill = false
		case tok == "--tool-change":
			o.OutputToolChange = true
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
	return o
}

// unit returns the active length unit code and label.
func (o GRBLOptions) unit() (code, format string) {
	if o.Inches {
		return "G20", "in"
	}
	return "G21", "mm"
}

// grblRenderer carries the GRBL export state: the running line number, the current tool
// position (needed to translate canned cycles), and the active retract mode.
type grblRenderer struct {
	opts         GRBLOptions
	lineNR       int
	curX, curY   float64
	curZ         float64
	drillRetract string // "G98" (return to prior Z) | "G99" (return to R)
}

// ExportGRBL renders objects to GRBL-dialect G-code, the Go port of grbl_legacy_post.export.
// Validated against FreeCAD's TestGrblLegacyPost.
func ExportGRBL(objects []Object, argstring string) string {
	r := &grblRenderer{opts: parseGRBLArgs(argstring), lineNR: 100, drillRetract: "G98"}
	var b strings.Builder
	r.writeHeaderAndPreamble(&b)
	for _, obj := range objects {
		r.writeOperation(&b, obj)
	}
	r.writePostamble(&b)
	return b.String()
}

// lineNumber returns the current "Nxxx " prefix and advances the counter (GRBL returns the
// number BEFORE incrementing, unlike LinuxCNC).
func (r *grblRenderer) lineNumber() string {
	if !r.opts.OutputLineNumber {
		return ""
	}
	s := "N" + strconv.Itoa(r.lineNR) + " "
	r.lineNR += 10
	return s
}

// writeHeaderAndPreamble emits the optional header, the preamble, and — only when the
// preamble does not already set them — the motion-mode and units lines.
func (r *grblRenderer) writeHeaderAndPreamble(b *strings.Builder) {
	if r.opts.OutputHeader {
		b.WriteString(r.lineNumber() + "(Exported by Oblikovati)\n")
		b.WriteString(r.lineNumber() + "(Post Processor: grbl)\n")
		b.WriteString(r.lineNumber() + "(Output Time: generated)\n")
	}
	if r.opts.OutputComments {
		b.WriteString(r.lineNumber() + "(Begin preamble)\n")
	}
	for _, line := range strings.Split(strings.TrimRight(r.opts.Preamble, "\n"), "\n") {
		b.WriteString(r.lineNumber() + line + "\n")
	}
	if !strings.Contains(r.opts.Preamble, "G90") && !strings.Contains(r.opts.Preamble, "G91") {
		b.WriteString(r.lineNumber() + "G90\n")
	}
	code, _ := r.opts.unit()
	if !strings.Contains(r.opts.Preamble, "G21") && !strings.Contains(r.opts.Preamble, "G20") {
		b.WriteString(r.lineNumber() + code + "\n")
	}
}

// writeOperation emits one operation's comment framing (capitalised, GRBL style) and its
// parsed path.
func (r *grblRenderer) writeOperation(b *strings.Builder, obj Object) {
	if r.opts.OutputComments {
		b.WriteString(r.lineNumber() + "(Begin operation: " + obj.Label + ")\n")
	}
	b.WriteString(r.parsePath(obj))
	if r.opts.OutputComments {
		b.WriteString(r.lineNumber() + "(Finish operation: " + obj.Label + ")\n")
	}
}

// writePostamble emits the begin-postamble comment and the postamble block.
func (r *grblRenderer) writePostamble(b *strings.Builder) {
	if r.opts.OutputComments {
		b.WriteString(r.lineNumber() + "(Begin postamble)\n")
	}
	for _, line := range strings.Split(strings.TrimRight(r.opts.Postamble, "\n"), "\n") {
		b.WriteString(r.lineNumber() + line + "\n")
	}
}

// grblParams is GRBL's address output order.
var grblParams = []string{"X", "Y", "Z", "A", "B", "C", "U", "V", "W", "I", "J", "K", "F", "S", "T", "Q", "R", "L", "P"}

// rapidMoves are the commands GRBL treats as rapids (feed suppressed).
var rapidMoves = map[string]bool{"G0": true, "G00": true}

// parsePath renders one object's path: a "(Path: label)" comment then each command, with
// canned drill cycles translated to G0/G1 moves and tool changes commented out.
func (r *grblRenderer) parsePath(obj Object) string {
	var out strings.Builder
	lastCommand := ""
	if r.opts.OutputComments {
		out.WriteString(r.lineNumber() + "(Path: " + obj.Label + ")\n")
	}
	for _, c := range obj.Path.Commands {
		tokens := r.commandTokens(c, lastCommand)
		lastCommand = c.Name
		r.trackPosition(c)

		if r.opts.TranslateDrill && (c.Name == "G81" || c.Name == "G82" || c.Name == "G83") {
			out.WriteString(r.drillTranslate(tokens, c))
			continue
		}
		if r.opts.TranslateDrill && (c.Name == "G84" || c.Name == "G74") {
			out.WriteString(r.tapTranslate(tokens, c))
			continue
		}
		// When translating cycles, the canned-cycle return-mode/cancel codes are not valid
		// GRBL — comment them out (the upstream's SUPPRESS_COMMANDS), e.g. G80 → "( G80 )".
		if r.opts.TranslateDrill && (c.Name == "G80" || c.Name == "G98" || c.Name == "G99") {
			tokens = append([]string{"("}, append(tokens, ")")...)
		}
		r.applyToolChange(&out, c, &tokens)
		if len(tokens) >= 1 {
			out.WriteString(r.lineNumber() + formatOutstring(tokens) + "\n")
		}
	}
	return out.String()
}

// commandTokens builds the address tokens for one command (name + parameters in
// grblParams order), suppressing a repeated name under modal.
func (r *grblRenderer) commandTokens(c gcode.Command, lastCommand string) []string {
	tokens := []string{c.Name}
	if r.opts.Modal && c.Name == lastCommand {
		tokens = tokens[:0]
	}
	for _, p := range grblParams {
		if v, ok := c.Params[p]; ok {
			if tok, emit := r.formatParam(c.Name, p, v); emit {
				tokens = append(tokens, tok)
			}
		}
	}
	return tokens
}

// formatParam renders one address token, suppressing a feed on a rapid or a non-positive
// feed.
func (r *grblRenderer) formatParam(name, p string, v float64) (string, bool) {
	switch p {
	case "F":
		if rapidMoves[name] || v <= 0 {
			return "", false
		}
		return "F" + r.fixed(v), true
	case "S", "T":
		return p + strconv.Itoa(int(v)), true
	case "L", "P":
		return p + strconv.FormatFloat(v, 'g', -1, 64), true
	case "A", "B", "C":
		return p + r.fixed(v), true
	default: // X Y Z U V W I J K Q R (lengths)
		return p + r.fixed(r.toUnit(v)), true
	}
}

// trackPosition updates the current tool position from a motion command and the retract
// mode from G98/G99 — both needed by the drill-cycle translator.
func (r *grblRenderer) trackPosition(c gcode.Command) {
	if motionCommands[c.Name] {
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
	if c.Name == "G98" || c.Name == "G99" {
		r.drillRetract = c.Name
	}
}

// motionCommands are the moves that update the tracked tool position.
var motionCommands = map[string]bool{"G0": true, "G00": true, "G1": true, "G01": true, "G2": true, "G02": true, "G3": true, "G03": true}

// applyToolChange handles M6: GRBL ignores tool changes, so the line is commented out
// (unless --tool-change), with a leading "(Begin toolchange)" comment.
func (r *grblRenderer) applyToolChange(out *strings.Builder, c gcode.Command, tokens *[]string) {
	if c.Name != "M6" && c.Name != "M06" {
		return
	}
	if r.opts.OutputComments {
		out.WriteString(r.lineNumber() + "(Begin toolchange)\n")
	}
	if !r.opts.OutputToolChange {
		*tokens = append([]string{"("}, *tokens...)
		*tokens = append(*tokens, ")")
	}
}

// toUnit converts mm to the output unit.
func (r *grblRenderer) toUnit(mm float64) float64 {
	if r.opts.Inches {
		return mm / 25.4
	}
	return mm
}

// fixed formats with the options' precision.
func (r *grblRenderer) fixed(v float64) string {
	return fmt.Sprintf("%.*f", r.opts.Precision, v)
}

// formatOutstring joins tokens with a space and trims — GRBL lines carry no trailing
// space (unlike LinuxCNC). Mirrors the upstream format_outstring.
func formatOutstring(tokens []string) string {
	return strings.TrimSpace(strings.Join(tokens, " "))
}
