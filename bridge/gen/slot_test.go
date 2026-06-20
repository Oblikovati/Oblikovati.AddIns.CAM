// SPDX-License-Identifier: GPL-2.0-only

package gen

import "testing"

// TestSlotSinglePass cuts a slot exactly the tool's width as one centreline pass on the boundary.
func TestSlotSinglePass(t *testing.T) {
	cmds, err := GenerateSlot(square(20), []float64{0}, testFeeds, SlotParams{ToolRadius: 2, Width: 4, Climb: true})
	if err != nil {
		t.Fatalf("GenerateSlot: %v", err)
	}
	if got := countPlunges(cmds); got != 1 {
		t.Errorf("a tool-width slot should be one pass, got %d", got)
	}
	// the single pass runs on the boundary centreline → 20×20 = 400.
	if a := cutPolygon(cmds).Area(); !approx(a, 400) {
		t.Errorf("centreline pass area = %g, want 400 (on the boundary)", a)
	}
}

// TestSlotWidePasses cuts a wider slot with symmetric side passes including the centreline.
func TestSlotWidePasses(t *testing.T) {
	// width 10, tool ⌀4 → halfClear = 3; at 0.75 step-over (3mm) → offsets -3,0,3 → 3 passes.
	cmds, err := GenerateSlot(square(40), []float64{0}, testFeeds, SlotParams{ToolRadius: 2, Width: 10, StepOver: 0.75, Climb: true})
	if err != nil {
		t.Fatalf("GenerateSlot: %v", err)
	}
	if got := countPlunges(cmds); got != 3 {
		t.Errorf("wide slot passes = %d, want 3 (centre + two sides)", got)
	}
	// passes run inner→outer, so the first is the boundary shrunk by halfClear (3) → 34×34 = 1156.
	if a := cutPolygon(cmds).Area(); !approx(a, 1156) {
		t.Errorf("first (inner) pass area = %g, want 1156 (34×34)", a)
	}
}

// TestSlotErrors covers the degenerate tool/width cases.
func TestSlotErrors(t *testing.T) {
	if _, err := GenerateSlot(square(20), []float64{0}, testFeeds, SlotParams{ToolRadius: 0, Width: 4}); err == nil {
		t.Error("a zero tool radius must error")
	}
	if _, err := GenerateSlot(square(20), []float64{0}, testFeeds, SlotParams{ToolRadius: 3, Width: 4}); err == nil {
		t.Error("a slot narrower than the tool must error")
	}
}
