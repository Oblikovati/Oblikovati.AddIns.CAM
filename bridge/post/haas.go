// SPDX-License-Identifier: GPL-2.0-only

package post

import "strings"

// ExportHaas renders objects to Haas-dialect G-code: a "%" wrapper, a five-digit O-number header,
// a safe-start block that cancels stray modal state (G40 G49 G80), each operation's blocks (canned
// cycles preserved), a G28 Z-home return, and M30. Built on the shared isoRenderer (see iso.go).
func ExportHaas(objects []Object, argstring string) string {
	o := parseISOArgs(argstring, isoOptions{OutputComments: true, SequenceNumbers: true, Precision: 4, ProgramNumber: 1, WorkOffset: "G54", UseTLO: true})
	r := &isoRenderer{opts: o, seq: 10}

	var b strings.Builder
	b.WriteString("%\n")
	r.line(&b, r.programLine(5))
	// Haas safe-start: rapid mode, plane, units, cutter-comp/length-comp/canned-cycle cancel,
	// absolute, and the first work offset.
	r.line(&b, "G00 G17 "+r.unitCode()+" G40 G49 G80 G90 "+workOffsetOr(o.WorkOffset))
	for _, obj := range objects {
		r.writeOperation(&b, obj)
	}
	r.line(&b, "M05")
	r.line(&b, "G28 G91 Z0.")
	r.line(&b, "G90")
	r.line(&b, "M30")
	b.WriteString("%\n")
	return b.String()
}
