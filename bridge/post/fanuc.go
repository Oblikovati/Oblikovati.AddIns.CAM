// SPDX-License-Identifier: GPL-2.0-only

package post

import "strings"

// ExportFanuc renders objects to Fanuc/ISO-dialect G-code: a "%" wrapper, a four-digit O-number
// header, a metric/absolute/feed-per-minute preamble, each operation's blocks (canned cycles
// preserved), and M30. Built on the shared isoRenderer (see iso.go).
func ExportFanuc(objects []Object, argstring string) string {
	o := parseISOArgs(argstring, isoOptions{OutputComments: true, SequenceNumbers: true, Precision: 3, ProgramNumber: 1})
	if o.Inches {
		o.Precision = 4
	}
	r := &isoRenderer{opts: o, seq: 10}

	var b strings.Builder
	b.WriteString("%\n")
	r.line(&b, r.programLine(4))
	r.line(&b, "G17 "+r.unitCode()+" G90 G94 G54")
	for _, obj := range objects {
		r.writeOperation(&b, obj)
	}
	r.line(&b, "M05")
	r.line(&b, "M30")
	b.WriteString("%\n")
	return b.String()
}
