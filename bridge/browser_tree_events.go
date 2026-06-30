// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/api/wire"
)

// ShowBrowserTree declares (or refreshes) the CAM model-browser pane from the current job.
func (e *Engine) ShowBrowserTree() (wire.OKResult, error) {
	e.mu.Lock()
	job := e.lastJob
	e.mu.Unlock()
	return e.api.Browser().SetPane(wire.BrowserPaneSpec{
		ID: CAMBrowserPaneID, Title: "CAM", Nodes: buildJobTreeNodes(job, e.bodyNames()),
	})
}

// showBrowserTreeAction opens the CAM Job tree (the Show CAM Tree command).
func (e *Engine) showBrowserTreeAction() (*JobResult, error) {
	if _, err := e.ShowBrowserTree(); err != nil {
		return nil, err
	}
	return &JobResult{Summary: "CAM: Job tree open."}, nil
}

// bodyNames maps a body index to its document name (sized to cover every index), for the Model
// nodes. Best-effort: a failed query yields no names and the tree falls back to "Body N".
func (e *Engine) bodyNames() []string {
	list, err := e.api.Body().List()
	if err != nil || len(list.Bodies) == 0 {
		return nil
	}
	maxIdx := 0
	for _, b := range list.Bodies {
		if b.Index > maxIdx {
			maxIdx = b.Index
		}
	}
	names := make([]string, maxIdx+1)
	for _, b := range list.Bodies {
		names[b.Index] = b.Name
	}
	return names
}

// handleBrowserNode dispatches a CAM tree interaction: double-click opens an editor, a
// context-menu choice runs the matching action. Other gestures (select/expand) need no work.
func (e *Engine) handleBrowserNode(node, gesture, menuItem string) {
	switch gesture {
	case "double":
		e.browserDoubleClick(node)
	case "menu":
		e.browserMenu(node, menuItem)
	}
}

// browserDoubleClick opens the operation editor for an op node, or the job panel for the Job node.
func (e *Engine) browserDoubleClick(node string) {
	if idx, ok := opIndexOf(node); ok {
		e.setEditingOp(idx)
		e.launchRun(e.showOperationEditorAction)
		return
	}
	if node == "job" {
		e.launchRun(e.showJobEditAction)
	}
}

// browserMenu runs the action behind a chosen context-menu item, keyed by the node it was on.
func (e *Engine) browserMenu(node, item string) {
	if idx, ok := opIndexOf(node); ok {
		e.opMenuAction(idx, item)
		return
	}
	switch {
	case node == "job":
		e.jobMenuAction(item)
	case node == "model" && item == "edit":
		e.launchRun(e.showModelSelectAction)
	case node == "stock" && item == "edit":
		e.launchRun(e.showJobEditAction) // Stock is edited on the Job Edit Setup tab
	case isToolNode(node):
		e.toolMenuAction(node, item)
	}
}

// toolMenuAction runs a tool node's menu action: edit opens the tool-controller editor on that
// tool, remove drops the last library tool.
func (e *Engine) toolMenuAction(node, item string) {
	switch item {
	case "edit":
		e.setEditingTool(toolIndexOf(node))
		e.launchRun(e.showToolEditAction)
	case "remove":
		e.runAndRefreshTree(e.removeToolAction)
	}
}

// toolIndexOf parses a "tool:N" node id (0 when it doesn't match).
func toolIndexOf(node string) int {
	var idx int
	if _, err := fmt.Sscanf(node, "tool:%d", &idx); err == nil {
		return idx
	}
	return 0
}

// opMenuAction targets operation idx and runs its menu action, refreshing the tree after.
func (e *Engine) opMenuAction(idx int, item string) {
	e.setEditingOp(idx)
	actions := map[string]func() (*JobResult, error){
		"edit": e.showOperationEditorAction, "toggle": e.toggleOpAction,
		"up": e.moveOpUpAction, "down": e.moveOpDownAction,
		"dup": e.duplicateOpAction, "del": e.deleteOpAction,
	}
	if action, ok := actions[item]; ok {
		e.runAndRefreshTree(action)
	}
}

// jobMenuAction runs the Job node's menu action.
func (e *Engine) jobMenuAction(item string) {
	switch item {
	case "edit":
		e.launchRun(e.showJobEditAction)
	case "regen":
		e.runAndRefreshTree(e.regenerateAction)
	case "post":
		e.launchRun(e.saveGCodeAction)
	case "simulate":
		e.launchRun(e.simulateAction)
	}
}

// runAndRefreshTree runs an action on the job goroutine and re-declares the tree afterwards so
// the browser reflects the change (a deleted/reordered/toggled operation).
func (e *Engine) runAndRefreshTree(action func() (*JobResult, error)) {
	e.launchRun(func() (*JobResult, error) {
		res, err := action()
		_, _ = e.ShowBrowserTree()
		return res, err
	})
}

// setEditingOp points the operation editor at index idx (the tree's selection target).
func (e *Engine) setEditingOp(idx int) {
	e.mu.Lock()
	e.editingOp = idx
	e.mu.Unlock()
}

// opIndexOf parses an "op:N" node id.
func opIndexOf(node string) (int, bool) {
	var idx int
	if _, err := fmt.Sscanf(node, "op:%d", &idx); err == nil {
		return idx, true
	}
	return 0, false
}

// isToolNode reports whether node is a "tool:N" id.
func isToolNode(node string) bool {
	var idx int
	_, err := fmt.Sscanf(node, "tool:%d", &idx)
	return err == nil
}
