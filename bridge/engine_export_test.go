// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"oblikovati.org/api/wire"
)

// TestSaveGCodeNeedsProgram errors when nothing has been posted yet.
func TestSaveGCodeNeedsProgram(t *testing.T) {
	if _, err := NewEngine(&recordingHost{}).saveGCodeAction(); err == nil {
		t.Error("saving with no posted G-code must error")
	}
}

// TestSaveGCodeOpensDialog posts a job then opens the save dialog.
func TestSaveGCodeOpensDialog(t *testing.T) {
	h := &recordingHost{}
	e := NewEngine(h)
	if _, err := e.RunDrillingJobOnHost(0); err != nil {
		t.Fatalf("RunDrillingJobOnHost: %v", err)
	}
	if _, err := e.saveGCodeAction(); err != nil {
		t.Fatalf("saveGCodeAction: %v", err)
	}
	if !h.called(wire.MethodDialogsShowFileDialog) {
		t.Errorf("save G-code should open the file dialog; got %v", h.methods)
	}
}

// TestChosenFile parses the dialog id + path from a file-dialog event.
func TestChosenFile(t *testing.T) {
	mk := func(id string, cancelled bool, paths ...string) []byte {
		b, _ := json.Marshal(wire.FileDialogChosenEvent{Type: wire.EventFileDialogChosen, ID: id, Cancelled: cancelled, Paths: paths})
		return b
	}
	if id, p, ok := chosenFile(mk(GCodeDialogID, false, "/tmp/a.nc")); !ok || p != "/tmp/a.nc" || id != GCodeDialogID {
		t.Errorf("valid event should yield its id+path, got (%q,%q,%v)", id, p, ok)
	}
	if _, _, ok := chosenFile(mk(GCodeDialogID, true, "/tmp/a.nc")); ok {
		t.Error("a cancelled event must be ignored")
	}
	if _, _, ok := chosenFile(mk(GCodeDialogID, false)); ok {
		t.Error("an event with no path must be ignored")
	}
}

// TestWriteGCode writes the remembered program to disk.
func TestWriteGCode(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.rememberGCode("G0 X0\nG1 Z-1\n")
	path := filepath.Join(t.TempDir(), "out.nc")
	e.writeGCode(path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(data) != "G0 X0\nG1 Z-1\n" {
		t.Errorf("written G-code = %q", string(data))
	}
}
