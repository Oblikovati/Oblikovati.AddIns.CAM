// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"strconv"
	"strings"

	"oblikovati.org/api/client"
	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
	"oblikovati.org/cam/bridge/feeds"
)

// CAMPanelID is the stable dockable-window id the CAM add-in owns.
const CAMPanelID = "com.oblikovati.cam.panel"

// ShowPanel creates (or replaces) the CAM dockable window: the post-processor choice, the
// drilling plunge feed, and a Generate button. Edits arrive as panel.valueChanged events
// (applyPanelEdit).
func (e *Engine) ShowPanel() (wire.OKResult, error) {
	e.mu.Lock()
	postName, feed, cut, body, material, flutes := e.postName, e.plungFeed, e.cut, e.targetBody, e.material, e.flutes
	spinUp, workOffset := e.spinUpSecs, e.workOffset
	e.mu.Unlock()
	return e.api.DockableWindows().Set(wire.DockableWindowSpec{
		ID:      CAMPanelID,
		Title:   "CAM",
		Dock:    types.DockRight,
		Visible: true,
		Controls: []wire.PanelControlSpec{
			client.PanelLabel("hdr", "— CAM job —"),
			client.PanelButton("newjob", "New Job…", NewJobCommandID),
			client.PanelDropdown("post", "Post processor", postOptions(), postName),
			client.PanelTextBox("work_offset", "Work offset (1=G54)", strconv.Itoa(workOffsetOrOne(workOffset))),
			client.PanelTextBox("body", "Body index", strconv.Itoa(body)),
			client.PanelTextBox("plunge_feed", "Feed (mm/min)", num(feed)),
			client.PanelDropdown("material", "Material (feeds & speeds)", feeds.Materials(), material),
			client.PanelTextBox("tool_dia", "Tool ⌀ (mm)", num(cut.ToolDiameter)),
			client.PanelTextBox("flutes", "Flutes", strconv.Itoa(flutes)),
			client.PanelTextBox("spin_up", "Spin-up (s)", num(spinUp)),
			client.PanelTextBox("step_down", "Step-down (mm)", num(cut.StepDown)),
			client.PanelTextBox("step_over", "Step-over (×⌀)", num(cut.StepOver)),
			client.PanelTextBox("cut_depth", "Cut depth (mm, 0=thru)", num(cut.CutDepth)),
			client.PanelTextBox("stock_xy", "Stock margin XY (mm)", num(cut.StockXYMargin)),
			client.PanelTextBox("stock_top", "Stock margin top (mm)", num(cut.StockTopMargin)),
			client.PanelTextBox("clearance", "Clearance above (mm)", num(cut.ClearanceAbove)),
			client.PanelTextBox("retract", "Retract above (mm)", num(cut.RetractAbove)),
			client.PanelSeparator(),
			client.PanelButton("drill", "Drilling", GenerateJobCommandID),
			client.PanelButton("profile", "Profile", GenerateProfileCommandID),
			client.PanelButton("pocket", "Pocket", GeneratePocketCommandID),
			client.PanelButton("adaptive", "Adaptive", GenerateAdaptiveCommandID),
			client.PanelButton("rest", "Rest", GenerateRestCommandID),
			client.PanelButton("trochoidal", "Trochoidal", GenerateTrochoidalCommandID),
			client.PanelButton("slot", "Slot", GenerateSlotCommandID),
			client.PanelButton("probe", "Probe", GenerateProbeCommandID),
			client.PanelButton("boreprobe", "Bore Probe", GenerateBoreProbeCommandID),
			client.PanelButton("bossprobe", "Boss Probe", GenerateBossProbeCommandID),
			client.PanelButton("toolprobe", "Tool Probe", GenerateToolProbeCommandID),
			client.PanelButton("helix", "Helix bore", GenerateHelixCommandID),
			client.PanelButton("thread", "Thread mill", GenerateThreadMillCommandID),
			client.PanelButton("counterbore", "Counterbore", GenerateCounterboreCommandID),
			client.PanelButton("tapping", "Tapping", GenerateTappingCommandID),
			client.PanelButton("countersink", "Countersink", GenerateCountersinkCommandID),
			client.PanelButton("face", "Face", GenerateMillFaceCommandID),
			client.PanelButton("engrave", "Engrave", GenerateEngraveCommandID),
			client.PanelButton("chamfer", "Chamfer", GenerateChamferCommandID),
			client.PanelButton("vcarve", "V-Carve", GenerateVCarveCommandID),
			client.PanelButton("surface", "3D Surface", GenerateSurfaceCommandID),
			client.PanelButton("crosshatch", "3D Crosshatch", GenerateCrosshatchCommandID),
			client.PanelButton("waterline", "Waterline", GenerateWaterlineCommandID),
			client.PanelButton("all", "Generate All", GenerateAllCommandID),
			client.PanelSeparator(),
			client.PanelButton("preview", "Preview profile", PreviewProfileCommandID),
			client.PanelButton("clearpreview", "Clear preview", ClearPreviewCommandID),
			client.PanelButton("tools", "Tools…", ShowToolsCommandID),
			client.PanelButton("ops", "Operations…", ShowOperationsCommandID),
			client.PanelButton("editop", "Edit Op…", EditOperationCommandID),
			client.PanelButton("savegcode", "Save G-code…", SaveGCodeCommandID),
		},
	})
}

// applyMaterialFeedsLocked recomputes the plunge feed and spindle speed from the selected
// material and tool diameter via the feeds & speeds calculator. The cutting feed is 3× the
// plunge, so the plunge is set to a third of the recommended cutting feed. The caller must hold
// e.mu; an unknown material / bad tool leaves the current feeds unchanged.
func (e *Engine) applyMaterialFeedsLocked() {
	rec, err := feeds.Recommend(e.material, e.cut.ToolDiameter, e.flutes)
	if err != nil {
		return
	}
	e.spindleSpeed = float64(rec.RPM)
	e.plungFeed = rec.FeedRate / 3
}

// applyPanelEdit writes one edited panel value back into the engine, keyed by control id.
func (e *Engine) applyPanelEdit(controlID, value string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	switch controlID {
	case "post":
		if v := strings.TrimSpace(value); v == "linuxcnc" || v == "grbl" || v == "fanuc" || v == "marlin" || v == "haas" || v == "heidenhain" {
			e.postName = v
		}
	case "body":
		if b := int(panelNum(value, float64(e.targetBody))); b >= 0 {
			e.targetBody = b
		}
	case "plunge_feed":
		e.plungFeed = panelNum(value, e.plungFeed)
	case "material":
		if _, ok := feeds.Lookup(value); ok {
			e.material = strings.ToLower(strings.TrimSpace(value))
			e.applyMaterialFeedsLocked()
		}
	case "tool_dia":
		if d := panelNum(value, e.cut.ToolDiameter); d > 0 {
			e.cut.ToolDiameter = d
			e.applyMaterialFeedsLocked() // a new diameter changes the recommended RPM/feed
		}
	case "flutes":
		if f := int(panelNum(value, float64(e.flutes))); f > 0 {
			e.flutes = f
			e.applyMaterialFeedsLocked() // more flutes → a faster feed at the same RPM
		}
	case "spin_up":
		if s := panelNum(value, e.spinUpSecs); s >= 0 {
			e.spinUpSecs = s
		}
	case "work_offset":
		if w := int(panelNum(value, float64(workOffsetOrOne(e.workOffset)))); w >= 1 && w <= 6 {
			e.workOffset = w
		}
	case "step_down":
		e.cut.StepDown = panelNum(value, e.cut.StepDown)
	case "step_over":
		if s := panelNum(value, e.cut.StepOver); s > 0 {
			e.cut.StepOver = s
		}
	case "cut_depth":
		e.cut.CutDepth = panelNum(value, e.cut.CutDepth)
	case "stock_xy":
		e.cut.StockXYMargin = panelNum(value, e.cut.StockXYMargin)
	case "stock_top":
		e.cut.StockTopMargin = panelNum(value, e.cut.StockTopMargin)
	case "clearance":
		e.cut.ClearanceAbove = panelNum(value, e.cut.ClearanceAbove)
	case "retract":
		e.cut.RetractAbove = panelNum(value, e.cut.RetractAbove)
	}
}

// num formats a parameter value for a panel text box (no trailing zeros).
func num(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

// panelNum reads the leading number from a form value (e.g. "120 mm/min" → 120), keeping the
// fallback when the field is empty or half-typed.
func panelNum(value string, fallback float64) float64 {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return fallback
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return fallback
	}
	return v
}
