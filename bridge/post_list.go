// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/post"
)

// PostObjects turns generated operation results into the labelled post objects the post
// processor renders, injecting the tool-change + spindle-start block (M6 Tn, then M3/M4 Sxxxx)
// before each operation that needs a fresh tool — the first op, and any op whose tool differs
// from the previous one. Consecutive operations sharing a tool keep the spindle running rather
// than re-selecting the same tool. A pared-down port of FreeCAD's Path/Post/PostList ordering:
// it makes the program self-contained (a machine sees the tool select and spindle start it
// needs before the cutting moves). The tool number/spindle come from the operation's controller.
func PostObjects(results []OperationResult) []post.Object {
	objects := make([]post.Object, 0, len(results)+1)
	if header, ok := toolListHeader(results); ok {
		objects = append(objects, header)
	}
	changes := toolChangeAt(results)
	for i, res := range results {
		var commands []gcode.Command
		if changes[i] {
			commands = append(commands, toolChangeBlock(res.Controller)...)
		}
		commands = append(commands, coolantOn(res.Coolant)...)
		commands = append(commands, res.Path.Commands...)
		commands = append(commands, coolantOff(res.Coolant)...)
		if res.PauseAfter {
			commands = append(commands, gcode.NewCommand("M1", nil)) // optional stop — inspect before continuing
		}
		objects = append(objects, post.Object{Label: res.Label, Path: gcode.NewPath(commands)})
	}
	return objects
}

// toolChangeAt reports, per operation, whether it needs a fresh tool change: the first operation
// always does, and thereafter only when its tool number differs from the previous operation's.
// Shared by the post (which emits the M6 block) and the cycle-time estimate (which charges the
// allowance) so the two agree on how many tool changes the program really has.
func toolChangeAt(results []OperationResult) []bool {
	flags := make([]bool, len(results))
	prev, have := 0, false
	for i, r := range results {
		if !have || r.Controller.ToolNumber != prev {
			flags[i] = true
			prev, have = r.Controller.ToolNumber, true
		}
	}
	return flags
}

// toolListHeader builds a comment-only "setup sheet" object listing the distinct tools the program
// uses (number, shape, diameter), in first-use order, so the operator knows what to load before
// running. Returns false when no operation carries a tool. The lines are whole-line comments, so
// every post renders them in its own comment style.
func toolListHeader(results []OperationResult) (post.Object, bool) {
	var lines []gcode.Command
	seen := map[int]bool{}
	for _, r := range results {
		if seen[r.Controller.ToolNumber] {
			continue
		}
		seen[r.Controller.ToolNumber] = true
		lines = append(lines, gcode.NewCommand(toolListLine(r.Controller), nil))
	}
	if len(lines) == 0 {
		return post.Object{}, false
	}
	return post.Object{Label: "Tool list", Path: gcode.NewPath(lines)}, true
}

// toolListLine formats one tool's setup-sheet comment, e.g. "(T1  endmill  D6.0mm)".
func toolListLine(tc ToolController) string {
	shape := tc.Tool.ShapeType
	if shape == "" {
		shape = "tool"
	}
	return fmt.Sprintf("(T%d  %s  D%.1fmm)", tc.ToolNumber, shape, tc.Tool.Diameter)
}

// coolantOn returns the coolant-start command for a mode (M8 flood, M7 mist), or nothing.
func coolantOn(mode string) []gcode.Command {
	switch mode {
	case CoolantFlood:
		return []gcode.Command{gcode.NewCommand("M8", nil)}
	case CoolantMist:
		return []gcode.Command{gcode.NewCommand("M7", nil)}
	default:
		return nil
	}
}

// coolantOff returns the coolant-stop command (M9) when a mode was on, or nothing.
func coolantOff(mode string) []gcode.Command {
	if mode == CoolantFlood || mode == CoolantMist {
		return []gcode.Command{gcode.NewCommand("M9", nil)}
	}
	return nil
}

// toolChangeBlock builds the leading M6/spindle commands for a controller: the tool select
// (M6 Tn) and, when the controller drives the spindle, the spindle start (M3/M4 Sxxxx). An
// empty spindle direction omits the spindle command.
func toolChangeBlock(tc ToolController) []gcode.Command {
	block := []gcode.Command{gcode.NewCommand("M6", map[string]float64{"T": float64(tc.ToolNumber)})}
	if spindle := tc.spindleM3M4(); spindle != "" {
		block = append(block, gcode.NewCommand(spindle, map[string]float64{"S": tc.SpindleSpeed}))
	}
	return block
}
