// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// TestDressupEditorSection checks an operation's holding-tabs dressup gets an editable section in
// the operation editor, with its parameters namespaced by dressup index.
func TestDressupEditorSection(t *testing.T) {
	op := &ProfileOp{OpBase: OpBase{OpLabel: "Outline"}}
	op.AppendDressup(NewTagsDressup(3, 4, 2))

	body := opEditorBody(op)
	if !hasGroupTitled(body, "Tags dress-up") {
		t.Fatal("missing the holding-tabs dress-up section")
	}
	tags, _ := findControl(body, func(c wire.PanelControlSpec) bool {
		return c.Kind == types.PanelGroup && c.Title == "Tags dress-up"
	})
	for _, id := range []string{"dr0_count", "dr0_width", "dr0_height"} {
		if _, ok := findControl(tags.Children, func(c wire.PanelControlSpec) bool { return c.ID == id }); !ok {
			t.Errorf("dress-up section missing %q", id)
		}
	}
}

// TestDressupParamEdit checks editing a namespaced dressup control updates that dressup's parameter.
func TestDressupParamEdit(t *testing.T) {
	op := &ProfileOp{OpBase: OpBase{OpLabel: "Outline"}}
	op.AppendDressup(NewTagsDressup(3, 4, 2))

	applyOpOrDressupEdit(op, "dr0_width", "8")

	tags, ok := op.DressupList()[0].(TagsDressup)
	if !ok || tags.Params.Width != 8 {
		t.Errorf("dressup width after edit = %+v, want 8", op.DressupList()[0])
	}
}

// TestParseDressupControl distinguishes dressup controls from operation parameters.
func TestParseDressupControl(t *testing.T) {
	if idx, param, ok := parseDressupControl("dr2_height"); !ok || idx != 2 || param != "height" {
		t.Errorf("parse(dr2_height) = %d,%q,%v", idx, param, ok)
	}
	for _, id := range []string{"drillDepth", "stepDown", "dr_x", "side"} {
		if _, _, ok := parseDressupControl(id); ok {
			t.Errorf("parse(%q) should not be a dressup control", id)
		}
	}
}
