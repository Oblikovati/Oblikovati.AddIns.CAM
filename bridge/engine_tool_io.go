// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"encoding/json"
	"fmt"
	"os"

	"oblikovati.org/api/wire"
)

// Dialog ids for tool-library file import/export, recognised in handleFileChosen.
const (
	ToolsExportDialogID = "com.oblikovati.cam.exporttools"
	ToolsImportDialogID = "com.oblikovati.cam.importtools"
)

// exportToolsAction opens a save dialog to write the tool library to a JSON file.
func (e *Engine) exportToolsAction() (*JobResult, error) {
	if _, err := e.api.Dialogs().ShowFileDialog(wire.ShowFileDialogArgs{
		ID: ToolsExportDialogID, Save: true, Title: "Export tool library", Filter: "Tool library (*.json)|*.json",
	}); err != nil {
		return nil, err
	}
	return &JobResult{Summary: "CAM: choose where to export the tool library…"}, nil
}

// importToolsAction opens an open dialog to load a tool library from a JSON file.
func (e *Engine) importToolsAction() (*JobResult, error) {
	if _, err := e.api.Dialogs().ShowFileDialog(wire.ShowFileDialogArgs{
		ID: ToolsImportDialogID, Title: "Import tool library", Filter: "Tool library (*.json)|*.json",
	}); err != nil {
		return nil, err
	}
	return &JobResult{Summary: "CAM: choose a tool library to import…"}, nil
}

// exportTools writes the current library to path as JSON and reports the outcome.
func (e *Engine) exportTools(path string) {
	e.mu.Lock()
	lib := e.library
	e.mu.Unlock()
	payload, err := json.MarshalIndent(lib, "", "  ")
	if err != nil {
		e.reportStatus("CAM: failed to encode tool library: " + err.Error())
		return
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		e.reportStatus("CAM: failed to export tool library: " + err.Error())
		return
	}
	e.reportStatus(fmt.Sprintf("CAM: exported %d tool(s) to %s", len(lib.Tools), path))
}

// importTools loads a tool library from path, replaces the current one, and refreshes the tool
// browser.
func (e *Engine) importTools(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		e.reportStatus("CAM: failed to read tool library: " + err.Error())
		return
	}
	lib, err := parseToolLibrary(data)
	if err != nil {
		e.reportStatus("CAM: " + err.Error())
		return
	}
	e.mu.Lock()
	e.library = lib
	e.mu.Unlock()
	if _, err := e.showToolLibrary(e.jobTools()); err != nil {
		e.reportStatus("CAM: imported tools but failed to refresh: " + err.Error())
		return
	}
	e.reportStatus(fmt.Sprintf("CAM: imported %d tool(s) from %s", len(lib.Tools), path))
}

// parseToolLibrary decodes a tool-library JSON document, requiring at least one tool.
func parseToolLibrary(data []byte) (ToolLibrary, error) {
	var lib ToolLibrary
	if err := json.Unmarshal(data, &lib); err != nil {
		return ToolLibrary{}, fmt.Errorf("tool library is not valid JSON: %w", err)
	}
	if len(lib.Tools) == 0 {
		return ToolLibrary{}, fmt.Errorf("tool library file has no tools")
	}
	return lib, nil
}
