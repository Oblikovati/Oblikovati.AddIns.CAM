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
	postName, feed := e.postName, e.plungFeed
	e.mu.Unlock()
	return e.api.DockableWindows().Set(wire.DockableWindowSpec{
		ID:      CAMPanelID,
		Title:   "CAM",
		Dock:    types.DockRight,
		Visible: true,
		Controls: []wire.PanelControlSpec{
			client.PanelLabel("hdr", "— CAM job —"),
			client.PanelDropdown("post", "Post processor", []string{"linuxcnc", "grbl"}, postName),
			client.PanelTextBox("plunge_feed", "Feed (mm/min)", strconv.FormatFloat(feed, 'g', -1, 64)),
			client.PanelSeparator(),
			client.PanelButton("drill", "Drilling", GenerateJobCommandID),
			client.PanelButton("profile", "Profile", GenerateProfileCommandID),
			client.PanelButton("pocket", "Pocket", GeneratePocketCommandID),
			client.PanelButton("helix", "Helix bore", GenerateHelixCommandID),
			client.PanelButton("face", "Face", GenerateMillFaceCommandID),
			client.PanelButton("engrave", "Engrave", GenerateEngraveCommandID),
			client.PanelSeparator(),
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
	}
}

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
