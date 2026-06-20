// SPDX-License-Identifier: GPL-2.0-only

package bridge

// cutSettings are the user-editable cutting parameters that drive every generated job,
// replacing the per-milestone hardcoded defaults. Distances are millimetres; StepOver is a
// fraction of the tool diameter (0..1). They are edited from the CAM panel and snapshotted into
// each job at generation time.
type cutSettings struct {
	ToolDiameter   float64 // end-mill / ball-nose diameter (mm)
	StepDown       float64 // max Z per pass (mm)
	StepOver       float64 // fraction of the tool diameter between passes (0..1)
	CutDepth       float64 // depth below the stock top to cut to (mm); <= 0 means through the stock
	StockXYMargin  float64 // extra material around the part in X/Y (mm)
	StockTopMargin float64 // extra material on top of the part (mm)
	ClearanceAbove float64 // rapid/clearance plane above the stock top (mm)
	RetractAbove   float64 // feed-in / canned-cycle retract plane above the stock top (mm)
}

// clearanceZ / retractZ return the rapid and retract planes above a stock top, falling back to
// the milestone defaults when a margin is non-positive.
func (c cutSettings) clearanceZ(topZ float64) float64 {
	return topZ + positiveOr(c.ClearanceAbove, drillClearanceAbove)
}
func (c cutSettings) retractZ(topZ float64) float64 {
	return topZ + positiveOr(c.RetractAbove, drillRetractAbove)
}

// positiveOr returns v when positive, else the fallback.
func positiveOr(v, fallback float64) float64 {
	if v > 0 {
		return v
	}
	return fallback
}

// defaultCutSettings are the milestone defaults, matched to the original hardcoded values so
// existing behaviour is unchanged until the user edits a field.
func defaultCutSettings() cutSettings {
	return cutSettings{
		ToolDiameter: millEndmillDia, StepDown: millStepDown, StepOver: 0.5, CutDepth: 0,
		ClearanceAbove: drillClearanceAbove, RetractAbove: drillRetractAbove,
	}
}

// cutting returns a snapshot of the current cutting settings (taken under the engine lock so a
// job reads a consistent set even while the panel is being edited).
func (e *Engine) cutting() cutSettings {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cut
}

// stockFor builds the job stock from a body's range box (cm), grown by the configured stock
// margins.
func (e *Engine) stockFor(min, max []float64) Stock {
	return e.cutting().grow(StockFromRangeBox(min, max))
}

// stepOverDistance converts the step-over fraction to an absolute distance (mm) for the tool.
func (c cutSettings) stepOverDistance() float64 { return c.StepOver * c.ToolDiameter }

// passSpacing returns the absolute pass spacing (mm) a 3D op samples at — the step-over
// fraction times the selected tool's diameter — falling back to a default when that would be
// non-positive (so a half-typed field never produces zero or negative scan lines).
func passSpacing(c cutSettings, toolDiameter, fallback float64) float64 {
	if d := c.StepOver * toolDiameter; d > 0 {
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

// grow expands a tight part-extent stock by the configured margins: extra material all around
// in X/Y and extra on top (the part still sits on the stock bottom). Zero margins leave a tight
// billet.
func (c cutSettings) grow(s Stock) Stock {
	s.Min.X -= c.StockXYMargin
	s.Min.Y -= c.StockXYMargin
	s.Max.X += c.StockXYMargin
	s.Max.Y += c.StockXYMargin
	s.Max.Z += c.StockTopMargin
	return s
}
