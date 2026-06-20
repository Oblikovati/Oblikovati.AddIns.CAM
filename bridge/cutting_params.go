// SPDX-License-Identifier: GPL-2.0-only

package bridge

// cutSettings are the user-editable cutting parameters that drive every generated job,
// replacing the per-milestone hardcoded defaults. Distances are millimetres; StepOver is a
// fraction of the tool diameter (0..1). They are edited from the CAM panel and snapshotted into
// each job at generation time.
type cutSettings struct {
	ToolDiameter float64 // end-mill / ball-nose diameter (mm)
	StepDown     float64 // max Z per pass (mm)
	StepOver     float64 // fraction of the tool diameter between passes (0..1)
	CutDepth     float64 // depth below the stock top to cut to (mm); <= 0 means through the stock
}

// defaultCutSettings are the milestone defaults, matched to the original hardcoded values so
// existing behaviour is unchanged until the user edits a field.
func defaultCutSettings() cutSettings {
	return cutSettings{ToolDiameter: millEndmillDia, StepDown: millStepDown, StepOver: 0.5, CutDepth: 0}
}

// cutting returns a snapshot of the current cutting settings (taken under the engine lock so a
// job reads a consistent set even while the panel is being edited).
func (e *Engine) cutting() cutSettings {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cut
}

// stepOverDistance converts the step-over fraction to an absolute distance (mm) for the tool.
func (c cutSettings) stepOverDistance() float64 { return c.StepOver * c.ToolDiameter }

// passSpacing returns the absolute pass/grid spacing (mm) the 3D ops sample at — the
// step-over distance, falling back to a default when the settings would give a non-positive
// spacing (so a half-typed field never produces zero or negative scan lines).
func passSpacing(c cutSettings, fallback float64) float64 {
	if d := c.stepOverDistance(); d > 0 {
		return d
	}
	return fallback
}

// levelSpacing returns the Z spacing (mm) between waterline levels — the step-down, falling
// back to a default when it would be non-positive.
func levelSpacing(c cutSettings, fallback float64) float64 {
	if c.StepDown > 0 {
		return c.StepDown
	}
	return fallback
}

// finalDepth returns the cut's bottom Z (mm) given the stock: a positive CutDepth cuts that far
// below the stock top, otherwise the cut goes through to the stock bottom.
func (c cutSettings) finalDepth(topZ, bottomZ float64) float64 {
	if c.CutDepth > 0 {
		return topZ - c.CutDepth
	}
	return bottomZ
}
