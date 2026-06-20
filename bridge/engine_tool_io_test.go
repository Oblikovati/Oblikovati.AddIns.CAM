// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"os"
	"path/filepath"
	"testing"

	"oblikovati.org/api/wire"
)

// TestToolLibraryFileRoundTrip exports the library to disk and imports it back.
func TestToolLibraryFileRoundTrip(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.library.add(newTool("endmill")) // a distinctive extra tool
	want := len(e.library.Tools)
	path := filepath.Join(t.TempDir(), "tools.json")

	e.exportTools(path)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("export did not write the file: %v", err)
	}

	e.library = ToolLibrary{} // wipe, then import
	e.importTools(path)
	if len(e.library.Tools) != want {
		t.Errorf("imported library has %d tools, want %d", len(e.library.Tools), want)
	}
}

// TestParseToolLibraryRejectsEmpty rejects an empty or malformed library file.
func TestParseToolLibraryRejectsEmpty(t *testing.T) {
	if _, err := parseToolLibrary([]byte(`{"tools":[]}`)); err == nil {
		t.Error("an empty tool library must be rejected")
	}
	if _, err := parseToolLibrary([]byte(`not json`)); err == nil {
		t.Error("invalid JSON must be rejected")
	}
}

// TestImportBadFileReports leaves the library unchanged when the file is missing.
func TestImportBadFileReports(t *testing.T) {
	e := NewEngine(&recordingHost{})
	before := len(e.library.Tools)
	e.importTools(filepath.Join(t.TempDir(), "nope.json")) // missing file
	if len(e.library.Tools) != before {
		t.Error("a failed import must not change the library")
	}
}

// TestExportToolsOpensDialog opens the save dialog.
func TestExportToolsOpensDialog(t *testing.T) {
	h := &recordingHost{}
	if _, err := NewEngine(h).exportToolsAction(); err != nil {
		t.Fatalf("exportToolsAction: %v", err)
	}
	if !h.called(wire.MethodDialogsShowFileDialog) {
		t.Error("export tools should open the file dialog")
	}
}

// TestImportToolsOpensDialog opens the open dialog.
func TestImportToolsOpensDialog(t *testing.T) {
	h := &recordingHost{}
	if _, err := NewEngine(h).importToolsAction(); err != nil {
		t.Fatalf("importToolsAction: %v", err)
	}
	if !h.called(wire.MethodDialogsShowFileDialog) {
		t.Error("import tools should open the file dialog")
	}
}

// TestHandleFileChosenIgnoresOthers does nothing for a cancelled or unknown-id event.
func TestHandleFileChosenIgnoresOthers(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.handleFileChosen([]byte(`{"type":"dialog.fileChosen","id":"other","paths":["/tmp/x"]}`)) // unknown id
	e.handleFileChosen([]byte(`{"type":"dialog.fileChosen","cancelled":true}`))                // cancelled
	e.handleFileChosen([]byte(`not json`))                                                     // garbage
}
