// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/api/client"
	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// ToolsBrowserID is the stable dockable-window id of the CAM tool-library browser.
const ToolsBrowserID = "com.oblikovati.cam.tools"

// showToolLibraryAction opens (or refreshes) the tool-library browser.
func (e *Engine) showToolLibraryAction() (*JobResult, error) {
	tools := e.jobTools()
	if _, err := e.showToolLibrary(tools); err != nil {
		return nil, err
	}
	return &JobResult{Summary: fmt.Sprintf("CAM: tool library open (%d tool(s)).", len(tools))}, nil
}

// showToolLibrary builds the tool-library dockable window: one row per loaded tool (the primary
// end mill plus the library tools) and buttons to add/remove tools.
func (e *Engine) showToolLibrary(tools []ToolController) (wire.OKResult, error) {
	controls := []wire.PanelControlSpec{client.PanelLabel("hdr", "— Tool library —")}
	for i, tc := range tools {
		controls = append(controls, client.PanelLabel(fmt.Sprintf("t%d", i), toolRow(tc)))
	}
	controls = append(controls,
		client.PanelSeparator(),
		client.PanelButton("add_em", "Add End Mill", AddEndmillCommandID),
		client.PanelButton("add_dr", "Add Drill", AddDrillCommandID),
		client.PanelButton("add_bn", "Add Ball-nose", AddBallnoseCommandID),
		client.PanelButton("rm", "Remove Tool", RemoveToolCommandID),
		client.PanelSeparator(),
		client.PanelButton("export", "Export Library…", ExportToolsCommandID),
		client.PanelButton("import", "Import Library…", ImportToolsCommandID),
	)
	return e.api.DockableWindows().Set(wire.DockableWindowSpec{
		ID: ToolsBrowserID, Title: "CAM Tools", Dock: types.DockLeft, Visible: true, Controls: controls,
	})
}

// toolRow formats one tool-library row: "T2 Drill 5mm — drill ⌀5 / F100".
func toolRow(tc ToolController) string {
	return fmt.Sprintf("T%d %s — %s ⌀%g / F%g", tc.ToolNumber, tc.Label, tc.Tool.ShapeType, tc.Tool.Diameter, tc.VertFeed)
}

// addToolAction adds a default tool of the given cutter shape to the library, persists it, and
// refreshes the browser.
func (e *Engine) addToolAction(shape string) (*JobResult, error) {
	e.mu.Lock()
	e.library.add(newTool(shape))
	e.mu.Unlock()
	_ = e.SaveToolLibrary() // best-effort: persist when a document is open
	tools := e.jobTools()
	if _, err := e.showToolLibrary(tools); err != nil {
		return nil, err
	}
	return &JobResult{Summary: fmt.Sprintf("CAM: added a %s; library has %d tool(s).", shape, len(tools))}, nil
}

// removeToolAction drops the last library tool, persists, and refreshes the browser.
func (e *Engine) removeToolAction() (*JobResult, error) {
	e.mu.Lock()
	removed := e.library.removeLast()
	e.mu.Unlock()
	if !removed {
		return &JobResult{Summary: "CAM: tool library is empty (the primary end mill always stays)."}, nil
	}
	_ = e.SaveToolLibrary()
	tools := e.jobTools()
	if _, err := e.showToolLibrary(tools); err != nil {
		return nil, err
	}
	return &JobResult{Summary: fmt.Sprintf("CAM: removed a tool; library has %d tool(s).", len(tools))}, nil
}

// newTool returns a sensible default tool controller for a cutter shape (its number is assigned
// by the library on add).
func newTool(shape string) ToolController {
	switch shape {
	case "drill":
		return ToolController{Label: "Drill 3mm", SpindleSpeed: 2500, SpindleDir: "Forward",
			VertFeed: 90, HorizFeed: 90, Tool: ToolBit{Name: "Drill 3mm", ShapeType: "drill", Diameter: 3, Flutes: 2}}
	case "ballend":
		return ToolController{Label: "Ball-nose 3mm", SpindleSpeed: 8000, SpindleDir: "Forward",
			VertFeed: 80, HorizFeed: 240, Tool: ToolBit{Name: "Ball-nose 3mm", ShapeType: "ballend", Diameter: 3, Flutes: 2}}
	default: // endmill
		return ToolController{Label: "End mill 4mm", SpindleSpeed: 7000, SpindleDir: "Forward",
			VertFeed: 90, HorizFeed: 270, Tool: ToolBit{Name: "End mill 4mm", ShapeType: "endmill", Diameter: 4, Flutes: 4}}
	}
}
