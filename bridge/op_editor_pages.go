// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"strings"

	"oblikovati.org/api/client"
	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// The operation editor laid out like FreeCAD's per-operation task pages: the operation's own
// parameters in a titled group, the shared depth/height/coolant parameters in a Depths & Heights
// group, then the action and dress-up button pads. Built on the grid/group layout (api v0.96.0).

// depthParamIDs are the parameter ids depthParams emits — the ones that belong on FreeCAD's
// Depths/Heights pages rather than the operation-specific page.
var depthParamIDs = map[string]bool{
	"startDepth": true, "finalDepth": true, "clearance": true,
	"coolant": true, "feedScale": true, "pauseAfter": true,
}

// opEditorBody groups an operation's parameters into FreeCAD-style pages: the op-specific ones by
// their Section (a parameter with no Section lands in the primary group, titled by op kind, listed
// first), the depth/height/coolant ones under "Depths & Heights" — each a 2-column form.
func opEditorBody(op Operation) []wire.PanelControlSpec {
	ed, ok := op.(Editable)
	if !ok {
		return []wire.PanelControlSpec{client.PanelLabel("noedit", "This operation has no editable parameters.")}
	}
	groups := newOpSectionGroups(operationKind(op))
	var depthFields []wire.PanelControlSpec
	for _, p := range ed.Parameters() {
		if depthParamIDs[p.ID] {
			depthFields = append(depthFields, paramControl(p))
			continue
		}
		groups.add(p.Section, paramControl(p))
	}
	out := groups.forms()
	if len(depthFields) > 0 {
		out = append(out, camForm("op_depths", "Depths & Heights", depthFields...))
	}
	return out
}

// opSectionGroups accumulates op-specific parameter controls into named sections, preserving the
// order sections are first seen (the primary group, titled by op kind, sorts first).
type opSectionGroups struct {
	primary string
	order   []string
	byTitle map[string][]wire.PanelControlSpec
}

func newOpSectionGroups(primary string) *opSectionGroups {
	return &opSectionGroups{primary: primary, byTitle: map[string][]wire.PanelControlSpec{}}
}

// add files a control under its section title (empty → the primary group).
func (g *opSectionGroups) add(section string, control wire.PanelControlSpec) {
	title := section
	if title == "" {
		title = g.primary
	}
	if _, seen := g.byTitle[title]; !seen {
		g.order = append(g.order, title)
	}
	g.byTitle[title] = append(g.byTitle[title], control)
}

// forms builds a 2-column form group per section, in first-seen order.
func (g *opSectionGroups) forms() []wire.PanelControlSpec {
	out := make([]wire.PanelControlSpec, 0, len(g.order))
	for _, title := range g.order {
		out = append(out, camForm("op_"+sectionID(title), title, g.byTitle[title]...))
	}
	return out
}

// sectionID makes a group id from a section title (lowercased, spaces → underscores).
func sectionID(title string) string {
	return strings.ToLower(strings.ReplaceAll(title, " ", "_"))
}

// opEditorActions is the operation editor's button pads: the Actions group (toggle / reorder /
// duplicate / delete), the Dress-ups group, and the Regenerate button — a 3-column grid each.
func opEditorActions() []wire.PanelControlSpec {
	thirds := []types.GridTrack{client.TrackFr(1), client.TrackFr(1), client.TrackFr(1)}
	actions := client.PanelGroup("op_act", "Actions",
		client.PanelGrid("op_act_g", thirds, 4, 4,
			client.PanelButton("toggle", "Enable/Disable", ToggleOpCommandID),
			client.PanelButton("up", "Move Up", MoveOpUpCommandID),
			client.PanelButton("down", "Move Down", MoveOpDownCommandID),
			client.PanelButton("dup", "Duplicate", DuplicateOpCommandID),
			client.PanelButton("addcustom", "Add Custom", AddCustomOpCommandID),
			client.PanelButton("del", "Delete", DeleteOpCommandID)))
	dressups := client.PanelGroup("op_dr", "Dress-ups",
		client.PanelGrid("op_dr_g", thirds, 4, 4,
			client.PanelButton("tabs", "Tabs", AddTabsCommandID),
			client.PanelButton("dogbone", "Dogbone", AddDogboneCommandID),
			client.PanelButton("ramp", "Ramp", AddRampCommandID),
			client.PanelButton("leadinout", "Lead In/Out", AddLeadInOutCommandID),
			client.PanelButton("helicalramp", "Helical Ramp", AddHelicalRampCommandID),
			client.PanelButton("cleardr", "Clear", ClearDressupsCommandID)))
	return []wire.PanelControlSpec{
		actions, dressups,
		client.PanelButton("regen", "Regenerate + Post", RegenerateCommandID),
	}
}
