// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/post"
)

// PostObjects turns generated operation results into the labelled post objects the post
// processor renders, injecting the tool-change + spindle-start block before each operation
// (M6 Tn, then M3/M4 Sxxxx). A pared-down port of FreeCAD's Path/Post/PostList ordering:
// it makes the program self-contained (a machine sees the tool select and spindle start it
// needs before the cutting moves). The tool number/spindle come from the operation's
// controller.
func PostObjects(results []OperationResult) []post.Object {
	objects := make([]post.Object, 0, len(results))
	for _, res := range results {
		commands := append(toolChangeBlock(res.Controller), res.Path.Commands...)
		objects = append(objects, post.Object{Label: res.Label, Path: gcode.NewPath(commands)})
	}
	return objects
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
