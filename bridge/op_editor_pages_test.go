// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// TestOpEditorBodyGroupsParams checks the operation editor groups parameters into a faithful
// Operation section and a Depths & Heights section (FreeCAD's PageOp / PageDepths / PageHeights),
// instead of one flat list.
func TestOpEditorBodyGroupsParams(t *testing.T) {
	op := &ProfileOp{OpBase: OpBase{OpLabel: "Outline", IsActive: true}}
	body := opEditorBody(op)

	if !hasGroupTitled(body, "Profile") {
		t.Error("missing the operation-parameters group titled by op kind")
	}
	if !hasGroupTitled(body, "Depths & Heights") {
		t.Error("missing the Depths & Heights group")
	}
	depths, _ := findControl(body, func(c wire.PanelControlSpec) bool {
		return c.Kind == types.PanelGroup && c.Title == "Depths & Heights"
	})
	if _, ok := findControl(depths.Children, func(c wire.PanelControlSpec) bool { return c.ID == "clearance" }); !ok {
		t.Error("clearance should be in the Depths & Heights group")
	}
	opg, _ := findControl(body, func(c wire.PanelControlSpec) bool {
		return c.Kind == types.PanelGroup && c.Title == "Profile"
	})
	if _, ok := findControl(opg.Children, func(c wire.PanelControlSpec) bool { return c.ID == "side" }); !ok {
		t.Error("side should be in the Operation group")
	}
}

// TestDrillingEditorSubSections checks an operation with a rich page (Drilling) is split into its
// FreeCAD sub-sections (Drilling / Peck / Dwell / Retract), not one flat operation group.
func TestDrillingEditorSubSections(t *testing.T) {
	op := &DrillingOp{OpBase: OpBase{OpLabel: "Drill"}}
	body := opEditorBody(op)
	for _, sec := range []string{"Drilling", "Peck", "Dwell", "Retract", "Depths & Heights"} {
		if !hasGroupTitled(body, sec) {
			t.Errorf("missing %q section", sec)
		}
	}
	peck, _ := findControl(body, func(c wire.PanelControlSpec) bool {
		return c.Kind == types.PanelGroup && c.Title == "Peck"
	})
	if _, ok := findControl(peck.Children, func(c wire.PanelControlSpec) bool { return c.ID == "peckDepth" }); !ok {
		t.Error("peckDepth should be in the Peck section")
	}
}
