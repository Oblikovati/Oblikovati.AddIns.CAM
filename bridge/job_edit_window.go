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
	post, material, outputFile, postArguments, orderBy                                         string
	feed, toolDia, stepDown, stepOver, cutDepth, stockXY, stockTop, clearance, retract, spinUp float64
	body, flutes, workOffset                                                                   int
	splitOutput                                                                                bool
	wcs                                                                                        map[int]bool
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
		outputFile: e.outputFile, postArguments: e.postArguments, orderBy: e.orderBy,
		splitOutput: e.splitOutput, wcs: selectedWCS(e.wcs, e.workOffset),
	}
}

// selectedWCS is the displayed work-coordinate-system selection: the engine's set, or — when
// none is stored yet — the active work offset (default G54) so the tab opens with one checked.
func selectedWCS(wcs map[int]bool, workOffset int) map[int]bool {
	out := map[int]bool{}
	any := false
	for k, v := range wcs {
		if v {
			out[k] = true
			any = true
		}
	}
	if !any {
		out[workOffsetOrOne(workOffset)] = true
	}
	return out
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

// outputTab is the Output tab: the output file, post processor and arguments, plus the
// Work Coordinate Systems group (the G54–G59 fixtures, output ordering and split-output) —
// FreeCAD's Output tab.
func outputTab(v jobEditValues) wire.PanelControlSpec {
	return client.PanelTab("Output",
		camForm("je_out", "Output",
			client.PanelTextBox("out_file", "Output file", v.outputFile),
			client.PanelDropdown("post", "Post processor", postOptions(), v.post),
			client.PanelTextBox("post_args", "Arguments", v.postArguments)),
		client.PanelGroup("je_wcs", "Work Coordinate Systems",
			wcsChecklist(v),
			client.PanelDropdown("order_by", "Order by", orderByOptions(), orderByOrFixture(v.orderBy)),
			client.PanelCheckBox("split_output", "Split output", v.splitOutput)),
	)
}

// wcsChecklist is the G54–G59 fixture checklist as a 3-column grid of checkboxes.
func wcsChecklist(v jobEditValues) wire.PanelControlSpec {
	thirds := []types.GridTrack{client.TrackFr(1), client.TrackFr(1), client.TrackFr(1)}
	var boxes []wire.PanelControlSpec
	for n := 1; n <= 6; n++ {
		boxes = append(boxes, client.PanelCheckBox(fmt.Sprintf("wcs_%d", n), fmt.Sprintf("G5%d", 3+n), v.wcs[n]))
	}
	return client.PanelGrid("je_wcs_g", thirds, 4, 4, boxes...)
}

// orderByOptions are the multi-fixture output orderings; orderByOrFixture defaults an unset value.
func orderByOptions() []string { return []string{"Fixture", "Tool", "Operation"} }

func orderByOrFixture(v string) string {
	if v == "Tool" || v == "Operation" {
		return v
	}
	return "Fixture"
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
