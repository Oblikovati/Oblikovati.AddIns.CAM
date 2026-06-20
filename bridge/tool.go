// SPDX-License-Identifier: GPL-2.0-only

package bridge

// ToolBit is the cutting tool's geometry: its diameter, a shape tag, and cutting-edge
// height. A pared-down port of FreeCAD's Path/Tool/toolbit (the milestone-1 drilling
// slice only needs the diameter and a type tag; the parametric shape comes later).
// Lengths are millimetres — the G-code/tooling convention.
type ToolBit struct {
	Name              string  // human label, e.g. "6mm Drill"
	ShapeType         string  // "drill" | "endmill" | "ballend" | … (informational in M1)
	Diameter          float64 // cutting diameter (mm)
	CuttingEdgeHeight float64 // flute / usable cutting length (mm)
}

// ToolController binds a ToolBit to its machining parameters — the spindle and feed/rapid
// rates an operation reads when it emits moves. Mirrors FreeCAD's Path/Tool/Controller
// (ToolNumber, SpindleSpeed/Dir, VertFeed/HorizFeed, VertRapid/HorizRapid). Feeds are
// mm/min; the spindle speed is rpm.
type ToolController struct {
	Label        string
	ToolNumber   int
	SpindleSpeed float64 // rpm
	SpindleDir   string  // "Forward" | "Reverse" | "None"
	VertFeed     float64 // plunge feed (mm/min)
	HorizFeed    float64 // cutting feed in the XY plane (mm/min)
	VertRapid    float64 // rapid plunge (mm/min); 0 = machine default (G0)
	HorizRapid   float64 // rapid traverse (mm/min); 0 = machine default (G0)
	Tool         ToolBit
}

// spindleM3M4 returns the spindle-start M code for the controller's direction: M3 forward,
// M4 reverse, empty when the spindle is left off. (M3/M4 per RS-274.)
func (tc ToolController) spindleM3M4() string {
	switch tc.SpindleDir {
	case "Reverse":
		return "M4"
	case "Forward":
		return "M3"
	default:
		return ""
	}
}
