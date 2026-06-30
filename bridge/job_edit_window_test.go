// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// TestJobEditWindowHasSixTabs checks the Job Edit window mirrors FreeCAD's tabbed editor:
// Setup / General / Output / Tools / Workplan / Advanced.
func TestJobEditWindowHasSixTabs(t *testing.T) {
	h := &recordingHost{}
	e := NewEngine(h)
	e.lastJob = &Job{Operations: []Operation{&ProfileOp{OpBase: OpBase{OpLabel: "P", IsActive: true}}}}

	if _, err := e.ShowJobEditWindow(); err != nil {
		t.Fatalf("ShowJobEditWindow: %v", err)
	}
	if len(h.dockWindows) == 0 {
		t.Fatal("Job Edit window not set")
	}
	win := h.dockWindows[len(h.dockWindows)-1]
	if win.ID != JobEditWindowID || win.Title != "Job Edit" {
		t.Errorf("window id/title = %q/%q", win.ID, win.Title)
	}
	tabsNode, found := findControl(win.Controls, func(c wire.PanelControlSpec) bool {
		return c.Kind == types.PanelTabs
	})
	if !found {
		t.Fatal("no tabs container")
	}
	want := []string{"Setup", "General", "Output", "Tools", "Workplan", "Advanced"}
	if len(tabsNode.Children) != len(want) {
		t.Fatalf("tabs = %d, want %d", len(tabsNode.Children), len(want))
	}
	for i, title := range want {
		if tabsNode.Children[i].Title != title {
			t.Errorf("tab %d = %q, want %q", i, tabsNode.Children[i].Title, title)
		}
	}
}
