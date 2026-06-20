// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestPanelEditsCuttingParams drives panel value edits and checks they land in the engine's
// cutting settings (and that junk/zero values are rejected).
func TestPanelEditsCuttingParams(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.applyPanelEdit("tool_dia", "8 mm")
	e.applyPanelEdit("step_down", "2")
	e.applyPanelEdit("step_over", "0.3")
	e.applyPanelEdit("cut_depth", "5 mm")
	got := e.cutting()
	if got.ToolDiameter != 8 || got.StepDown != 2 || got.StepOver != 0.3 || got.CutDepth != 5 {
		t.Fatalf("cutting settings after edits = %+v", got)
	}
	// a zero/negative tool diameter or step-over is rejected (keeps the last good value)
	e.applyPanelEdit("tool_dia", "0")
	e.applyPanelEdit("step_over", "-1")
	if g := e.cutting(); g.ToolDiameter != 8 || g.StepOver != 0.3 {
		t.Errorf("invalid edits changed state: %+v", g)
	}
}

// TestCutSettingsHelpers covers the derived quantities.
func TestCutSettingsHelpers(t *testing.T) {
	c := cutSettings{ToolDiameter: 6, StepDown: 3, StepOver: 0.5, CutDepth: 4}
	if c.stepOverDistance() != 3 {
		t.Errorf("stepOverDistance = %g, want 3 (0.5×6)", c.stepOverDistance())
	}
	if c.finalDepth(10, 0) != 6 { // 10 − 4
		t.Errorf("finalDepth with CutDepth=4 = %g, want 6", c.finalDepth(10, 0))
	}
	if (cutSettings{}).finalDepth(10, 0) != 0 { // CutDepth 0 → through to bottom
		t.Errorf("finalDepth with CutDepth=0 should be the stock bottom")
	}
	if passSpacing(cutSettings{}, 6, 1.5) != 1.5 {
		t.Errorf("passSpacing must fall back when step-over distance is zero")
	}
	if passSpacing(cutSettings{StepOver: 0.5}, 6, 1.5) != 3 {
		t.Errorf("passSpacing = step-over × diameter (0.5×6=3)")
	}
	if levelSpacing(cutSettings{}, 1.0) != 1.0 {
		t.Errorf("levelSpacing must fall back when step-down is zero")
	}
}

// TestProfileJobReflectsParams confirms the edited parameters reach a generated profile job.
func TestProfileJobReflectsParams(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.applyPanelEdit("tool_dia", "8")
	e.applyPanelEdit("step_down", "2")
	e.applyPanelEdit("cut_depth", "5") // stock top is 10 mm (max z 1 cm) → final depth 5

	job, _, err := e.buildProfileJob(0)
	if err != nil {
		t.Fatalf("buildProfileJob: %v", err)
	}
	if job.Tools[0].Tool.Diameter != 8 {
		t.Errorf("tool diameter = %g, want 8", job.Tools[0].Tool.Diameter)
	}
	op := job.Operations[0].(*ProfileOp)
	if op.StepDown != 2 {
		t.Errorf("profile step-down = %g, want 2", op.StepDown)
	}
	if op.FinalDepth != 5 {
		t.Errorf("profile final depth = %g, want 5 (10 mm top − 5 mm cut depth)", op.FinalDepth)
	}
}

// TestStockMargins grows the tight billet by the configured margins.
func TestStockMargins(t *testing.T) {
	c := cutSettings{StockXYMargin: 2, StockTopMargin: 3}
	s := c.grow(Stock{Min: vec(0, 0, 0), Max: vec(10, 6, 4)})
	if s.Min.X != -2 || s.Max.X != 12 || s.Max.Y != 8 || s.Max.Z != 7 || s.Min.Z != 0 {
		t.Errorf("grown stock = %+v, want XY±2 / top+3 / bottom unchanged", s)
	}
}

// TestPanelEditsStock routes the stock-margin panel fields.
func TestPanelEditsStock(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.applyPanelEdit("stock_xy", "2.5")
	e.applyPanelEdit("stock_top", "4")
	if g := e.cutting(); g.StockXYMargin != 2.5 || g.StockTopMargin != 4 {
		t.Errorf("stock margins after edits = %+v", g)
	}
}
