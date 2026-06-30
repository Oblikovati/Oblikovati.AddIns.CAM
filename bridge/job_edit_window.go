// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"strconv"

	"oblikovati.org/api/client"
	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
	"oblikovati.org/cam/bridge/feeds"
)

// JobEditWindowID is the stable id of the Job Edit window — FreeCAD's tabbed Job editor
// (Setup / General / Output / Tools / Workplan / Advanced), built on the grid/group/tabs layout
// (api v0.96.0). Its fields reuse the CAM panel's control ids, so edits route through the same
// applyPanelEdit. Opened by double-clicking the Job node or its "Edit Job…" menu.
const JobEditWindowID = "com.oblikovati.cam.jobedit"

// ShowJobEditWindow opens (or replaces) the Job Edit window from the current settings.
func (e *Engine) ShowJobEditWindow() (wire.OKResult, error) {
	v := e.jobEditValues()
	tabs := client.PanelTabs("jobedit_tabs",
		setupTab(v), generalTab(v), outputTab(v), toolsTab(v), e.workplanTab(), advancedTab(v))
	return e.api.DockableWindows().Set(wire.DockableWindowSpec{
		ID: JobEditWindowID, Title: "Job Edit", Dock: types.DockRight, Visible: true,
		Controls: []wire.PanelControlSpec{tabs},
	})
}

// showJobEditAction opens the Job Edit window (the Job node's edit gesture).
func (e *Engine) showJobEditAction() (*JobResult, error) {
	if _, err := e.ShowJobEditWindow(); err != nil {
		return nil, err
	}
	return &JobResult{Summary: "CAM: Job Edit open."}, nil
}

// jobEditValues is a lock-free snapshot of the settings the Job Edit window renders.
type jobEditValues struct {
	post, material                                                                             string
	feed, toolDia, stepDown, stepOver, cutDepth, stockXY, stockTop, clearance, retract, spinUp float64
	body, flutes, workOffset                                                                   int
}

// jobEditValues snapshots the engine settings under the lock.
func (e *Engine) jobEditValues() jobEditValues {
	e.mu.Lock()
	defer e.mu.Unlock()
	return jobEditValues{
		post: e.postName, material: e.material, feed: e.plungFeed,
		toolDia: e.cut.ToolDiameter, stepDown: e.cut.StepDown, stepOver: e.cut.StepOver,
		cutDepth: e.cut.CutDepth, stockXY: e.cut.StockXYMargin, stockTop: e.cut.StockTopMargin,
		clearance: e.cut.ClearanceAbove, retract: e.cut.RetractAbove, spinUp: e.spinUpSecs,
		body: e.targetBody, flutes: e.flutes, workOffset: e.workOffset,
	}
}

// camForm is a titled group whose body is a 2-column [label | field] grid: each field's caption
// (its Text) goes in the label column, the field renders bare in the value column.
func camForm(id, title string, fields ...wire.PanelControlSpec) wire.PanelControlSpec {
	cols := []types.GridTrack{client.TrackAuto(), client.TrackFr(1)}
	cells := make([]wire.PanelControlSpec, 0, len(fields)*2)
	for _, f := range fields {
		cells = append(cells, client.PanelLabel(f.ID+"_lbl", f.Text), f)
	}
	return client.PanelGroup(id, title, client.PanelGrid(id+"_grid", cols, 8, 4, cells...))
}

// setupTab is FreeCAD's Setup tab: Stock, Depths and Heights.
func setupTab(v jobEditValues) wire.PanelControlSpec {
	return client.PanelTab("Setup",
		camForm("je_stock", "Stock",
			client.PanelTextBox("stock_xy", "Stock margin XY (mm)", num(v.stockXY)),
			client.PanelTextBox("stock_top", "Stock margin top (mm)", num(v.stockTop))),
		camForm("je_depths", "Depths",
			client.PanelTextBox("step_down", "Step-down (mm)", num(v.stepDown)),
			client.PanelTextBox("cut_depth", "Cut depth (mm, 0=thru)", num(v.cutDepth))),
		camForm("je_heights", "Heights",
			client.PanelTextBox("clearance", "Clearance above (mm)", num(v.clearance)),
			client.PanelTextBox("retract", "Retract above (mm)", num(v.retract))),
	)
}

// generalTab is the General tab: the job's model body.
func generalTab(v jobEditValues) wire.PanelControlSpec {
	return client.PanelTab("General",
		camForm("je_job", "Job",
			client.PanelTextBox("body", "Body index", strconv.Itoa(v.body))),
		client.PanelButton("je_editmodel", "Edit Model…", ModelSelectCommandID),
	)
}

// outputTab is the Output tab: post processor and work-coordinate-system offset.
func outputTab(v jobEditValues) wire.PanelControlSpec {
	return client.PanelTab("Output",
		camForm("je_out", "Output",
			client.PanelDropdown("post", "Post processor", postOptions(), v.post),
			client.PanelTextBox("work_offset", "Work offset (1=G54)", strconv.Itoa(workOffsetOrOne(v.workOffset)))),
	)
}

// toolsTab is the Tools tab: the cutting tool and its feeds & speeds.
func toolsTab(v jobEditValues) wire.PanelControlSpec {
	return client.PanelTab("Tools",
		camForm("je_cut", "Cutting",
			client.PanelDropdown("material", "Material (feeds & speeds)", feeds.Materials(), v.material),
			client.PanelTextBox("tool_dia", "Tool ⌀ (mm)", num(v.toolDia)),
			client.PanelTextBox("flutes", "Flutes", strconv.Itoa(v.flutes)),
			client.PanelTextBox("plunge_feed", "Feed (mm/min)", num(v.feed)),
			client.PanelTextBox("spin_up", "Spin-up (s)", num(v.spinUp))),
		client.PanelButton("je_edittool", "Edit Tool Controller…", ToolEditCommandID),
	)
}

// workplanTab is the Workplan tab: the ordered operations and the row of edit/reorder/delete
// buttons (which act on the operations browser selection / operation editor).
func (e *Engine) workplanTab() wire.PanelControlSpec {
	e.mu.Lock()
	job := e.lastJob
	e.mu.Unlock()
	rows := operationListRows(job)
	rows = append(rows,
		client.PanelButton("je_editop", "Edit Op…", EditOperationCommandID),
		client.PanelButton("je_up", "Move Up", MoveOpUpCommandID),
		client.PanelButton("je_down", "Move Down", MoveOpDownCommandID),
		client.PanelButton("je_del", "Delete", DeleteOpCommandID))
	return client.PanelTab("Workplan", client.PanelGroup("je_ops", "Operations", rows...))
}

// operationListRows is one label row per operation, or a placeholder when there are none.
func operationListRows(job *Job) []wire.PanelControlSpec {
	if job == nil || len(job.Operations) == 0 {
		return []wire.PanelControlSpec{client.PanelLabel("je_noops", "No operations yet.")}
	}
	rows := make([]wire.PanelControlSpec, 0, len(job.Operations))
	for i, op := range job.Operations {
		rows = append(rows, client.PanelLabel(fmt.Sprintf("je_op%d", i), operationRow(i, op)))
	}
	return rows
}

// advancedTab is the Advanced tab: operation defaults (step-over).
func advancedTab(v jobEditValues) wire.PanelControlSpec {
	return client.PanelTab("Advanced",
		camForm("je_adv", "Operation Defaults",
			client.PanelTextBox("step_over", "Step-over (×⌀)", num(v.stepOver))),
	)
}

// postOptions are the post processors offered in the post-processor dropdowns.
func postOptions() []string {
	return []string{"linuxcnc", "grbl", "fanuc", "marlin", "haas", "heidenhain"}
}
