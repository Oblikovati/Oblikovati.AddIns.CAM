// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"strings"

	"oblikovati.org/cam/bridge/gcode"
)

// CustomOp emits operator-supplied raw G-code verbatim as a step in the job sequence — the escape
// hatch for moves the toolpath generators do not produce (a manual tool-change macro, a probing
// routine, an auxiliary M-function, a comment block). It ports the role of FreeCAD's CAM Custom
// op. The GCode is one command per line; blank lines are skipped. It takes the standard tool-change
// framing for its controller like any operation, so set its tool controller to the previous op's
// to keep the spindle running.
type CustomOp struct {
	OpBase
	GCode string // raw G-code, one command per line, emitted verbatim
}

// Features reports the property groups a custom op uses — only a tool controller (so it slots into
// the program with the right tool/spindle); the G-code itself is verbatim, not geometry-driven.
func (op *CustomOp) Features() FeatureFlag {
	return FeatureTool
}

// Execute parses the raw G-code lines into commands and returns them unframed — the operator's
// lines are emitted exactly as written. Errors when no non-blank line is present.
func (op *CustomOp) Execute(job *Job) (gcode.Path, error) {
	var cmds []gcode.Command
	for _, line := range strings.Split(op.GCode, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			cmds = append(cmds, gcode.ParseCommand(line))
		}
	}
	if len(cmds) == 0 {
		return gcode.Path{}, fmt.Errorf("custom operation %q has no G-code", op.OpLabel)
	}
	return gcode.NewPath(cmds), nil
}
