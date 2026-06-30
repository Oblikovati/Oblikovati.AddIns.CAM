// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/api/wire"
)

// CAMBrowserPaneID is the stable id of the CAM model-browser pane. The tree mirrors FreeCAD's
// Job tree — Job ▸ Model / Stock / Tools / Operations — with every node iconned (like the
// document tree) and right-clickable (api v0.97.0).
const CAMBrowserPaneID = "com.oblikovati.cam.tree"

// nodeIconModel / nodeIconStock are inline themed glyphs (the ribbon's #00ff00 backplate /
// #000 primary / #ff0000 accent convention) for the category nodes that have no command icon.
const (
	nodeIconModel = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="#000" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="1" y="1" width="22" height="22" rx="4" fill="#00ff00" stroke="none"/><path d="M12 4 L20 8 V16 L12 20 L4 16 V8 Z"/><path d="M4 8 L12 12 L20 8 M12 12 V20" stroke="#ff0000"/></svg>`
	nodeIconStock = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="#000" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="1" y="1" width="22" height="22" rx="4" fill="#00ff00" stroke="none"/><rect x="4" y="7" width="13" height="13" rx="1" stroke-dasharray="3 2"/><path d="M17 7 L20 4 M7 4 L20 4 L20 17" stroke="#ff0000"/></svg>`
)

// buildJobTreeNodes builds the CAM browser tree from the current job (nil → a placeholder
// prompting New Job). bodyNames maps a body index to its document name for the Model nodes.
func buildJobTreeNodes(job *Job, bodyNames []string) []wire.BrowserNodeSpec {
	if job == nil {
		return []wire.BrowserNodeSpec{{ID: "nojob", Label: "(no job — use New Job)"}}
	}
	return []wire.BrowserNodeSpec{{
		ID: "job", Label: "Job", IconSVG: iconSVG("newjob"), Expanded: true, Menu: jobMenuItems(),
		Children: []wire.BrowserNodeSpec{
			modelNode(job, bodyNames),
			stockNode(),
			toolsNode(job),
			opsNode(job),
		},
	}}
}

// modelNode lists the job's model solids by name.
func modelNode(job *Job, bodyNames []string) wire.BrowserNodeSpec {
	var kids []wire.BrowserNodeSpec
	for _, idx := range job.ModelBodies {
		kids = append(kids, wire.BrowserNodeSpec{
			ID: fmt.Sprintf("model:%d", idx), Label: bodyName(bodyNames, idx), IconSVG: nodeIconModel,
		})
	}
	return wire.BrowserNodeSpec{ID: "model", Label: "Model", IconSVG: nodeIconModel, Expanded: true, Menu: modelMenuItems(), Children: kids}
}

// stockNode is the raw-material node (edited on the Job Edit Setup tab).
func stockNode() wire.BrowserNodeSpec {
	return wire.BrowserNodeSpec{ID: "stock", Label: "Stock", IconSVG: nodeIconStock, Menu: stockMenuItems()}
}

// toolsNode lists the job's tool controllers.
func toolsNode(job *Job) wire.BrowserNodeSpec {
	var kids []wire.BrowserNodeSpec
	for i, tc := range job.Tools {
		kids = append(kids, wire.BrowserNodeSpec{
			ID: fmt.Sprintf("tool:%d", i), Label: toolLabel(i, tc), IconSVG: iconSVG("endmill"), Menu: toolMenuItems(),
		})
	}
	return wire.BrowserNodeSpec{ID: "tools", Label: "Tools", IconSVG: iconSVG("toollib"), Children: kids}
}

// opsNode lists the job's operations in order.
func opsNode(job *Job) wire.BrowserNodeSpec {
	var kids []wire.BrowserNodeSpec
	for i, op := range job.Operations {
		kids = append(kids, opNode(i, op))
	}
	return wire.BrowserNodeSpec{ID: "ops", Label: "Operations", IconSVG: iconSVG("generateall"), Expanded: true, Children: kids}
}

// opNode is one operation node, labelled by kind + label (and "(off)" when inactive), iconned by
// its operation glyph, and right-clickable for edit/reorder/delete.
func opNode(i int, op Operation) wire.BrowserNodeSpec {
	label := fmt.Sprintf("%s — %s", operationKind(op), op.Label())
	if !op.Active() {
		label += " (off)"
	}
	return wire.BrowserNodeSpec{
		ID: fmt.Sprintf("op:%d", i), Label: label, IconSVG: opNodeIcon(op), Menu: opMenuItems(),
	}
}

// bodyName is the document name of body idx, or a "Body N" fallback.
func bodyName(names []string, idx int) string {
	if idx >= 0 && idx < len(names) && names[idx] != "" {
		return names[idx]
	}
	return fmt.Sprintf("Body %d", idx)
}

// toolLabel labels a tool-controller node by its label or, failing that, its position.
func toolLabel(i int, tc ToolController) string {
	if tc.Label != "" {
		return tc.Label
	}
	return fmt.Sprintf("Tool %d", i+1)
}

// opNodeIcon resolves an operation's bundled glyph, falling back to the program glyph.
func opNodeIcon(op Operation) string {
	if key, ok := opKindIconKey[operationKind(op)]; ok {
		if svg := iconSVG(key); svg != "" {
			return svg
		}
	}
	return iconSVG("generateall")
}

// opKindIconKey maps an operation kind (operationKind) to its bundled ribbon glyph key.
var opKindIconKey = map[string]string{
	"Drilling": "drilling", "Profile": "profile", "Pocket": "pocket", "Adaptive": "adaptive",
	"Rest": "rest", "Trochoidal": "trochoidal", "Face": "face", "Engrave": "engrave",
	"Chamfer": "chamfer", "V-Carve": "vcarve", "Slot": "slot", "Probe": "probe",
	"Tool Probe": "toolprobe", "Helix": "helix", "Thread": "threadmill", "Counterbore": "counterbore",
	"Tapping": "tapping", "Custom": "customop", "Countersink": "countersink", "Surface": "surface",
	"Waterline": "waterline",
}

// jobMenuItems is the Job node's right-click menu.
func jobMenuItems() []wire.BrowserMenuItem {
	return []wire.BrowserMenuItem{
		{ID: "edit", Label: "Edit Job…"},
		{ID: "regen", Label: "Regenerate"},
		{ID: "post", Label: "Post Process (Save G-code)…"},
	}
}

// opMenuItems is an operation node's right-click menu (edit / reorder / duplicate / delete).
func opMenuItems() []wire.BrowserMenuItem {
	return []wire.BrowserMenuItem{
		{ID: "edit", Label: "Edit…"},
		{ID: "toggle", Label: "Enable/Disable"},
		{ID: "up", Label: "Move Up"},
		{ID: "down", Label: "Move Down"},
		{ID: "dup", Label: "Duplicate"},
		{ID: "del", Label: "Delete"},
	}
}

// toolMenuItems is a tool-controller node's right-click menu.
func toolMenuItems() []wire.BrowserMenuItem {
	return []wire.BrowserMenuItem{{ID: "edit", Label: "Edit…"}, {ID: "remove", Label: "Remove"}}
}

// stockMenuItems is the Stock node's right-click menu.
func stockMenuItems() []wire.BrowserMenuItem {
	return []wire.BrowserMenuItem{{ID: "edit", Label: "Edit Stock…"}}
}

// modelMenuItems is the Model node's right-click menu.
func modelMenuItems() []wire.BrowserMenuItem {
	return []wire.BrowserMenuItem{{ID: "edit", Label: "Edit Model…"}}
}
