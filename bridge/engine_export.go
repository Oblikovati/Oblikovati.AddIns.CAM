// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"oblikovati.org/api/wire"
)

// GCodeDialogID keys the save-file dialog the G-code export opens, so its chosen-file event is
// recognised in Notify.
const GCodeDialogID = "com.oblikovati.cam.savegcode"

// rememberGCode stores the most recently posted program for export.
func (e *Engine) rememberGCode(gcode string) {
	e.mu.Lock()
	e.lastGCode = gcode
	e.mu.Unlock()
}

// saveGCodeAction opens the host's save-file dialog for the last posted program; the chosen
// path arrives as a file-dialog event handled in Notify (handleFileChosen).
func (e *Engine) saveGCodeAction() (*JobResult, error) {
	e.mu.Lock()
	have := e.lastGCode != ""
	out := strings.TrimSpace(e.outputFile)
	e.mu.Unlock()
	if !have {
		return nil, fmt.Errorf("no G-code to save — generate a job first")
	}
	args := wire.ShowFileDialogArgs{ID: GCodeDialogID, Save: true, Title: "Save G-code", Filter: "G-code (*.nc)|*.nc"}
	if out != "" { // the Output tab's output file seeds the dialog's starting directory
		args.InitialDir = filepath.Dir(out)
	}
	if _, err := e.api.Dialogs().ShowFileDialog(args); err != nil {
		return nil, err
	}
	return &JobResult{Summary: "CAM: choose where to save the G-code…"}, nil
}

// handleFileChosen dispatches a file-dialog choice to the matching CAM file action (G-code
// export, tool-library export/import). Each action runs off the session goroutine.
func (e *Engine) handleFileChosen(ev []byte) {
	id, path, ok := chosenFile(ev)
	if !ok {
		return
	}
	switch id {
	case GCodeDialogID:
		go e.writeGCode(path)
	case ToolsExportDialogID:
		go e.exportTools(path)
	case ToolsImportDialogID:
		go e.importTools(path)
	}
}

// chosenFile extracts the dialog id and chosen path from a file-dialog event, reporting false
// when it was cancelled or carries no path.
func chosenFile(ev []byte) (id, path string, ok bool) {
	var chosen wire.FileDialogChosenEvent
	if json.Unmarshal(ev, &chosen) != nil || chosen.Cancelled || len(chosen.Paths) == 0 {
		return "", "", false
	}
	return chosen.ID, chosen.Paths[0], true
}

// writeGCode writes the last posted program to path and reports the outcome on the status bar.
// With split output, it writes one file per unit beside the chosen path instead.
func (e *Engine) writeGCode(path string) {
	e.mu.Lock()
	gcode := e.lastGCode
	programs := e.lastPrograms
	e.mu.Unlock()
	if len(programs) > 1 {
		e.writeSplitGCode(path, programs)
		return
	}
	if err := os.WriteFile(path, []byte(gcode), 0o644); err != nil {
		e.reportStatus("CAM: failed to save G-code: " + err.Error())
		return
	}
	e.reportStatus(fmt.Sprintf("CAM: saved %d bytes of G-code to %s", len(gcode), path))
}

// writeSplitGCode writes one file per output unit, named "<base>_<suffix><ext>" beside the chosen
// path (e.g. program_G54.nc, program_G55.nc).
func (e *Engine) writeSplitGCode(path string, programs []namedProgram) {
	dir := filepath.Dir(path)
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(filepath.Base(path), ext)
	for _, p := range programs {
		file := filepath.Join(dir, fmt.Sprintf("%s_%s%s", base, p.Suffix, ext))
		if err := os.WriteFile(file, []byte(p.GCode), 0o644); err != nil {
			e.reportStatus("CAM: failed to save split G-code: " + err.Error())
			return
		}
	}
	e.reportStatus(fmt.Sprintf("CAM: saved %d split G-code file(s) beside %s", len(programs), path))
}
