// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"encoding/json"
	"fmt"
	"os"

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
	e.mu.Unlock()
	if !have {
		return nil, fmt.Errorf("no G-code to save — generate a job first")
	}
	if _, err := e.api.Dialogs().ShowFileDialog(wire.ShowFileDialogArgs{
		ID: GCodeDialogID, Save: true, Title: "Save G-code", Filter: "G-code (*.nc)|*.nc",
	}); err != nil {
		return nil, err
	}
	return &JobResult{Summary: "CAM: choose where to save the G-code…"}, nil
}

// handleFileChosen writes the last posted program to the path the user picked in the G-code
// save dialog. The write + its status report run off the session goroutine.
func (e *Engine) handleFileChosen(ev []byte) {
	if path, ok := gcodeSavePath(ev); ok {
		go e.writeGCode(path)
	}
}

// gcodeSavePath extracts the chosen save path from a file-dialog event, reporting false when
// the event is not the G-code dialog's, was cancelled, or carries no path.
func gcodeSavePath(ev []byte) (string, bool) {
	var chosen wire.FileDialogChosenEvent
	if json.Unmarshal(ev, &chosen) != nil || chosen.ID != GCodeDialogID || chosen.Cancelled || len(chosen.Paths) == 0 {
		return "", false
	}
	return chosen.Paths[0], true
}

// writeGCode writes the last posted program to path and reports the outcome on the status bar.
func (e *Engine) writeGCode(path string) {
	e.mu.Lock()
	gcode := e.lastGCode
	e.mu.Unlock()
	if err := os.WriteFile(path, []byte(gcode), 0o644); err != nil {
		e.reportStatus("CAM: failed to save G-code: " + err.Error())
		return
	}
	e.reportStatus(fmt.Sprintf("CAM: saved %d bytes of G-code to %s", len(gcode), path))
}
