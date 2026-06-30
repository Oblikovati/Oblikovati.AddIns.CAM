// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/api/wire"
)

func findNode(nodes []wire.BrowserNodeSpec, id string) (wire.BrowserNodeSpec, bool) {
	for _, n := range nodes {
		if n.ID == id {
			return n, true
		}
		if got, ok := findNode(n.Children, id); ok {
			return got, true
		}
	}
	return wire.BrowserNodeSpec{}, false
}

// TestBuildJobTreeStructure checks the CAM browser tree mirrors FreeCAD's Job tree: a Job root
// nesting Model / Stock / Tools / Operations, with one node per operation, each carrying an icon
// and a right-click menu.
func TestBuildJobTreeStructure(t *testing.T) {
	job := &Job{
		ModelBodies: []int{0},
		Operations: []Operation{
			&ProfileOp{OpBase: OpBase{OpLabel: "Profile", IsActive: true}},
			&DrillingOp{OpBase: OpBase{OpLabel: "Drill", IsActive: false}},
		},
	}
	nodes := buildJobTreeNodes(job, []string{"Plate"})

	root, ok := findNode(nodes, "job")
	if !ok || root.Label != "Job" {
		t.Fatalf("missing Job root: %+v", nodes)
	}
	if root.IconSVG == "" {
		t.Error("Job node has no icon")
	}
	for _, cat := range []string{"model", "stock", "tools", "ops"} {
		if _, ok := findNode(nodes, cat); !ok {
			t.Errorf("missing %q category node", cat)
		}
	}
	// One node per operation, under Operations.
	opsNode, _ := findNode(nodes, "ops")
	if len(opsNode.Children) != 2 {
		t.Fatalf("operations children = %d, want 2", len(opsNode.Children))
	}
	op0 := opsNode.Children[0]
	if op0.ID != "op:0" || op0.IconSVG == "" || len(op0.Menu) == 0 {
		t.Errorf("op node missing id/icon/menu: %+v", op0)
	}
	// The model node lists the selected body by name.
	model, _ := findNode(nodes, "model")
	if len(model.Children) != 1 || model.Children[0].Label != "Plate" {
		t.Errorf("model children = %+v, want one named Plate", model.Children)
	}
}

// TestJobTreeOpMenuActions pins the operation context-menu actions used by the event dispatcher.
func TestJobTreeOpMenuActions(t *testing.T) {
	items := opMenuItems()
	want := map[string]bool{"edit": false, "toggle": false, "up": false, "down": false, "dup": false, "del": false}
	for _, it := range items {
		if _, ok := want[it.ID]; ok {
			want[it.ID] = true
		}
	}
	for id, seen := range want {
		if !seen {
			t.Errorf("operation menu missing %q action", id)
		}
	}
}

// TestOpIndexOf checks operation node ids parse and category nodes don't.
func TestOpIndexOf(t *testing.T) {
	if idx, ok := opIndexOf("op:3"); !ok || idx != 3 {
		t.Errorf("opIndexOf(op:3) = %d,%v", idx, ok)
	}
	for _, node := range []string{"model:0", "ops", "tool:1", "job"} {
		if _, ok := opIndexOf(node); ok {
			t.Errorf("opIndexOf(%q) should not match an operation", node)
		}
	}
	if !isToolNode("tool:2") || isToolNode("tools") {
		t.Error("isToolNode mismatch")
	}
}

// TestBrowserDoubleClickTargetsOp checks a double-click on an op node points the editor at it.
func TestBrowserDoubleClickTargetsOp(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.lastJob = &Job{Operations: []Operation{
		&ProfileOp{OpBase: OpBase{OpLabel: "a"}},
		&PocketOp{OpBase: OpBase{OpLabel: "b"}},
		&DrillingOp{OpBase: OpBase{OpLabel: "c"}},
	}}
	e.browserDoubleClick("op:2") // sets editingOp synchronously, then runs the editor async
	if e.editingOp != 2 {
		t.Errorf("editingOp = %d, want 2", e.editingOp)
	}
}
