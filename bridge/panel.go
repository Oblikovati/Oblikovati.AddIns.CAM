// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"strconv"
	"strings"

	"oblikovati.org/api/client"
	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// CAMPanelID is the stable dockable-window id the CAM add-in owns.
const CAMPanelID = "com.oblikovati.cam.panel"

// ShowPanel creates (or replaces) the CAM dockable window: the post-processor choice, the
// drilling plunge feed, and a Generate button. Edits arrive as panel.valueChanged events
// (applyPanelEdit).
func (e *Engine) ShowPanel() (wire.OKResult, error) {
	e.mu.Lock()
	postName, feed, cut := e.postName, e.plungFeed, e.cut
	e.mu.Unlock()
	return e.api.DockableWindows().Set(wire.DockableWindowSpec{
		ID:      CAMPanelID,
		Title:   "CAM",
		Dock:    types.DockRight,
		Visible: true,
		Controls: []wire.PanelControlSpec{
			client.PanelLabel("hdr", "— CAM job —"),
			client.PanelDropdown("post", "Post processor", []string{"linuxcnc", "grbl"}, postName),
			client.PanelTextBox("plunge_feed", "Feed (mm/min)", num(feed)),
			client.PanelTextBox("tool_dia", "Tool ⌀ (mm)", num(cut.ToolDiameter)),
			client.PanelTextBox("step_down", "Step-down (mm)", num(cut.StepDown)),
			client.PanelTextBox("step_over", "Step-over (×⌀)", num(cut.StepOver)),
			client.PanelTextBox("cut_depth", "Cut depth (mm, 0=thru)", num(cut.CutDepth)),
			client.PanelSeparator(),
			client.PanelButton("drill", "Drilling", GenerateJobCommandID),
			client.PanelButton("profile", "Profile", GenerateProfileCommandID),
			client.PanelButton("pocket", "Pocket", GeneratePocketCommandID),
			client.PanelButton("helix", "Helix bore", GenerateHelixCommandID),
			client.PanelButton("face", "Face", GenerateMillFaceCommandID),
			client.PanelButton("engrave", "Engrave", GenerateEngraveCommandID),
			client.PanelButton("surface", "3D Surface", GenerateSurfaceCommandID),
			client.PanelButton("waterline", "Waterline", GenerateWaterlineCommandID),
			client.PanelButton("all", "Generate All", GenerateAllCommandID),
			client.PanelSeparator(),
			client.PanelButton("preview", "Preview profile", PreviewProfileCommandID),
			client.PanelButton("clearpreview", "Clear preview", ClearPreviewCommandID),
			client.PanelButton("tools", "Tools…", ShowToolsCommandID),
			client.PanelButton("ops", "Operations…", ShowOperationsCommandID),
		},
	})
}

// applyPanelEdit writes one edited panel value back into the engine, keyed by control id.
func (e *Engine) applyPanelEdit(controlID, value string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	switch controlID {
	case "post":
		if v := strings.TrimSpace(value); v == "linuxcnc" || v == "grbl" {
			e.postName = v
		}
	case "plunge_feed":
		e.plungFeed = panelNum(value, e.plungFeed)
	case "tool_dia":
		if d := panelNum(value, e.cut.ToolDiameter); d > 0 {
			e.cut.ToolDiameter = d
		}
	case "step_down":
		e.cut.StepDown = panelNum(value, e.cut.StepDown)
	case "step_over":
		if s := panelNum(value, e.cut.StepOver); s > 0 {
			e.cut.StepOver = s
		}
	case "cut_depth":
		e.cut.CutDepth = panelNum(value, e.cut.CutDepth)
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
