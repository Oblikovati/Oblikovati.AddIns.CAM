// SPDX-License-Identifier: GPL-2.0-only

// Package post renders a toolpath (gcode.Path) to machine G-code. Each post processor is a
// pure command-list → string transform with no host or kernel dependency, mirroring
// FreeCAD's Path/Post scripts. Milestone 1 ports two legacy posts — LinuxCNC and GRBL —
// validated against FreeCAD's exact-string oracle tests.
package post

import (
	"fmt"
	"strings"

	"oblikovati.org/cam/bridge/gcode"
)

// Export renders objects with the named post processor ("linuxcnc" | "grbl"), erroring on
// an unknown name (the message lists the supported posts).
func Export(name string, objects []Object, argstring string) (string, error) {
	switch name {
	case "linuxcnc", "":
		return ExportLinuxCNC(objects, argstring), nil
	case "grbl":
		return ExportGRBL(objects, argstring), nil
	default:
		return "", fmt.Errorf("unknown post processor %q (supported: linuxcnc, grbl)", name)
	}
}

// Object is one postable item: a labelled toolpath. A post renders a sequence of Objects
// (operations), wrapping each in the controller-specific operation framing. It is the
// minimal stand-in for FreeCAD's "postable" doc objects.
type Object struct {
	Label string
	Path  gcode.Path
}

// shlexSplit splits a command-argument string the way Python's shlex.split does for the
// subset the posts use: whitespace-separated tokens, with single or double quotes grouping
// a token and suppressing the whitespace inside. Quotes are removed; backslash escapes are
// left literal (the posts do their own `\n` un-escaping on preamble/postamble values).
func shlexSplit(s string) []string {
	var tokens []string
	var cur strings.Builder
	inToken := false
	quote := rune(0)
	flush := func() {
		if inToken {
			tokens = append(tokens, cur.String())
			cur.Reset()
			inToken = false
		}
	}
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			inToken = true
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		default:
			cur.WriteRune(r)
			inToken = true
		}
	}
	flush()
	return tokens
}
