// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
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

// opEditorBody groups an operation's parameters: the op-specific ones under a group titled by the
// operation kind, the depth/height/coolant ones under "Depths & Heights" — each a 2-column form.
func opEditorBody(op Operation) []wire.PanelControlSpec {
	ed, ok := op.(Editable)
	if !ok {
		return []wire.PanelControlSpec{client.PanelLabel("noedit", "This operation has no editable parameters.")}
	}
	var opFields, depthFields []wire.PanelControlSpec
	for _, p := range ed.Parameters() {
		control := paramControl(p)
		if depthParamIDs[p.ID] {
			depthFields = append(depthFields, control)
		} else {
			opFields = append(opFields, control)
		}
	}
	var groups []wire.PanelControlSpec
	if len(opFields) > 0 {
		groups = append(groups, camForm("op_params", operationKind(op), opFields...))
	}
	if len(depthFields) > 0 {
		groups = append(groups, camForm("op_depths", "Depths & Heights", depthFields...))
	}
	return groups
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
