// SPDX-License-Identifier: GPL-2.0-only

package bridge

// ToolLibrary is the set of tool controllers the add-in keeps loaded beyond the primary milling
// end mill (which the CAM panel quick-edits): the drill, the ball-nose finisher, and any tools
// the user adds. Operations pick the controller matching their cutter shape, so a multi-operation
// program emits the right tool changes — a tool library plus per-job tool controllers.
type ToolLibrary struct {
	Tools []ToolController `json:"tools"`
}

// DefaultToolLibrary is the starter set: a 5 mm drill and a 6 mm ball-nose finisher. The primary
// end mill is supplied separately by the engine from the panel's tool/feed fields (tool T1).
func DefaultToolLibrary() ToolLibrary {
	return ToolLibrary{Tools: []ToolController{
		{Label: "Drill 5mm", ToolNumber: 2, SpindleSpeed: 2000, SpindleDir: "Forward",
			VertFeed: 100, HorizFeed: 100, Tool: ToolBit{Name: "Drill 5mm", ShapeType: "drill", Diameter: 5, Flutes: 2}},
		{Label: "Ball-nose 6mm", ToolNumber: 3, SpindleSpeed: 6000, SpindleDir: "Forward",
			VertFeed: 90, HorizFeed: 270, Tool: ToolBit{Name: "Ball-nose 6mm", ShapeType: "ballend", Diameter: 6, Flutes: 2}},
	}}
}

// snapshot returns a copy of the library's controllers so a job holds its own slice.
func (l ToolLibrary) snapshot() []ToolController {
	out := make([]ToolController, len(l.Tools))
	copy(out, l.Tools)
	return out
}

// add appends a tool, assigning it the next free tool number.
func (l *ToolLibrary) add(tc ToolController) {
	tc.ToolNumber = l.nextNumber()
	l.Tools = append(l.Tools, tc)
}

// removeLast drops the most recently added tool, reporting whether one was removed (the library
// may be emptied; the primary end mill always remains available from the engine).
func (l *ToolLibrary) removeLast() bool {
	if len(l.Tools) == 0 {
		return false
	}
	l.Tools = l.Tools[:len(l.Tools)-1]
	return true
}

// nextNumber returns one past the highest tool number in use (the primary end mill holds T1, so
// the floor is 2).
func (l ToolLibrary) nextNumber() int {
	max := 1
	for _, t := range l.Tools {
		if t.ToolNumber > max {
			max = t.ToolNumber
		}
	}
	return max + 1
}

// indexForShape returns the index of the first tool in tools whose cutter shape matches, or 0
// (the primary end mill) when none matches — so an operation always resolves to some tool.
func indexForShape(tools []ToolController, shape string) int {
	for i, t := range tools {
		if t.Tool.ShapeType == shape {
			return i
		}
	}
	return 0
}
