// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestDefaultToolLibrary checks the starter library has a drill and a ball-nose.
func TestDefaultToolLibrary(t *testing.T) {
	lib := DefaultToolLibrary()
	if indexForShape(lib.Tools, "drill") < 0 || indexForShape(lib.Tools, "ballend") < 0 {
		t.Fatalf("default library should contain a drill and a ball-nose: %+v", lib.Tools)
	}
}

// TestLibraryAddAssignsNumbers checks added tools get increasing free tool numbers.
func TestLibraryAddAssignsNumbers(t *testing.T) {
	lib := DefaultToolLibrary() // T2 drill, T3 ball-nose
	lib.add(newTool("endmill"))
	last := lib.Tools[len(lib.Tools)-1]
	if last.ToolNumber != 4 {
		t.Errorf("added tool number = %d, want 4 (after T3)", last.ToolNumber)
	}
	if last.Tool.ShapeType != "endmill" {
		t.Errorf("added tool shape = %q, want endmill", last.Tool.ShapeType)
	}
}

// TestLibraryRemoveLast removes the most recent tool and reports emptiness.
func TestLibraryRemoveLast(t *testing.T) {
	lib := ToolLibrary{}
	if lib.removeLast() {
		t.Error("removing from an empty library must report false")
	}
	lib.add(newTool("drill"))
	if !lib.removeLast() || len(lib.Tools) != 0 {
		t.Error("removeLast should drop the only tool and report true")
	}
}

// TestJobToolsSelection checks a job loads the end mill + library and resolves each shape.
func TestJobToolsSelection(t *testing.T) {
	tools := NewEngine(&recordingHost{}).jobTools()
	if tools[0].Tool.ShapeType != "endmill" || tools[0].ToolNumber != 1 {
		t.Errorf("tool 0 should be the primary end mill (T1), got %+v", tools[0])
	}
	if d := indexForShape(tools, "drill"); tools[d].Tool.ShapeType != "drill" {
		t.Errorf("no drill resolved in %+v", tools)
	}
	if b := indexForShape(tools, "ballend"); tools[b].Tool.ShapeType != "ballend" {
		t.Errorf("no ball-nose resolved in %+v", tools)
	}
}

// TestToolLibraryPersistence round-trips the library through the document attribute store.
func TestToolLibraryPersistence(t *testing.T) {
	e := NewEngine(&persistHost{})
	e.library.add(newTool("endmill")) // a non-default tool to detect on reload
	want := len(e.library.Tools)
	if err := e.SaveToolLibrary(); err != nil {
		t.Fatalf("SaveToolLibrary: %v", err)
	}
	e.library = ToolLibrary{} // wipe, then reload from the store
	if err := e.LoadToolLibrary(); err != nil {
		t.Fatalf("LoadToolLibrary: %v", err)
	}
	if len(e.library.Tools) != want {
		t.Errorf("library after reload has %d tools, want %d", len(e.library.Tools), want)
	}
}

// TestToolBrowserActions covers opening the browser and adding/removing tools.
func TestToolBrowserActions(t *testing.T) {
	e := NewEngine(&recordingHost{})
	if _, err := e.showToolLibraryAction(); err != nil {
		t.Fatalf("showToolLibraryAction: %v", err)
	}
	before := len(e.library.Tools)
	if _, err := e.addToolAction("ballend"); err != nil {
		t.Fatalf("addToolAction: %v", err)
	}
	if len(e.library.Tools) != before+1 {
		t.Errorf("add did not grow the library: %d → %d", before, len(e.library.Tools))
	}
	if _, err := e.removeToolAction(); err != nil {
		t.Fatalf("removeToolAction: %v", err)
	}
	if len(e.library.Tools) != before {
		t.Errorf("remove did not shrink the library back to %d, got %d", before, len(e.library.Tools))
	}
}
