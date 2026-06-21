// SPDX-License-Identifier: GPL-2.0-only

// Package post renders a toolpath (gcode.Path) to machine G-code. Each post processor is a
// pure command-list → string transform with no host or kernel dependency, mirroring
// FreeCAD's Path/Post scripts. Milestone 1 ports two legacy posts — LinuxCNC and GRBL —
// validated against FreeCAD's exact-string oracle tests.
package post

import (
	"fmt"
	"strconv"
	"strings"

	"oblikovati.org/cam/bridge/gcode"
)

// parseScalarPostArgs reads the comment / inch / precision flags shared by the self-contained
// posts (Marlin, Heidenhain), starting from the given default precision. Inches force 4-decimal
// precision, matching the legacy posts. Posts with extra flags parse those on top of this.
func parseScalarPostArgs(argstring string, defaultPrecision int) (comments, inches bool, precision int) {
	comments, precision = true, defaultPrecision
	for _, tok := range shlexSplit(argstring) {
		switch {
		case tok == "--no-comments":
			comments = false
		case tok == "--inches":
			inches = true
		case strings.HasPrefix(tok, "--precision="):
			if p, err := strconv.Atoi(strings.TrimPrefix(tok, "--precision=")); err == nil {
				precision = p
			}
		}
	}
	if inches {
		precision = 4
	}
	return comments, inches, precision
}

// Export renders objects with the named post processor ("linuxcnc" | "grbl" | "fanuc" | "marlin"
// | "haas" | "heidenhain"), erroring on an unknown name (the message lists the supported posts).
func Export(name string, objects []Object, argstring string) (string, error) {
	switch name {
	case "linuxcnc", "":
		return ExportLinuxCNC(objects, argstring), nil
	case "grbl":
		return ExportGRBL(objects, argstring), nil
	case "fanuc":
		return ExportFanuc(objects, argstring), nil
	case "marlin":
		return ExportMarlin(objects, argstring), nil
	case "haas":
		return ExportHaas(objects, argstring), nil
	case "heidenhain":
		return ExportHeidenhain(objects, argstring), nil
	default:
		return "", fmt.Errorf("unknown post processor %q (supported: linuxcnc, grbl, fanuc, marlin, haas, heidenhain)", name)
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
