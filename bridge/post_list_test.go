// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// totalM6 counts M6 commands in the posted objects' command lists.
func totalM6(results []OperationResult) int {
	n := 0
	for _, obj := range PostObjects(results) {
		for _, c := range obj.Path.Commands {
			if c.Name == "M6" {
				n++
			}
		}
	}
	return n
}

// TestPostSuppressesRedundantToolChange checks consecutive operations on the same tool emit only
// one tool change, while a tool switch in between emits another.
func TestPostSuppressesRedundantToolChange(t *testing.T) {
	t1 := ToolController{ToolNumber: 1, SpindleSpeed: 5000, SpindleDir: "Forward"}
	t2 := ToolController{ToolNumber: 2, SpindleSpeed: 3000, SpindleDir: "Forward"}

	// Two ops on the same tool → one tool change.
	same := []OperationResult{
		{Label: "Pocket", Path: NewJobPath("G1 X1"), Controller: t1},
		{Label: "Profile", Path: NewJobPath("G1 X2"), Controller: t1},
	}
	if got := totalM6(same); got != 1 {
		t.Errorf("two ops on the same tool should emit one M6, got %d", got)
	}

	// Tool 1, tool 2, tool 1 → three changes (each transition is a real swap).
	alternating := []OperationResult{
		{Label: "Drill", Path: NewJobPath("G1 X1"), Controller: t1},
		{Label: "Mill", Path: NewJobPath("G1 X2"), Controller: t2},
		{Label: "Drill2", Path: NewJobPath("G1 X3"), Controller: t1},
	}
	if got := totalM6(alternating); got != 3 {
		t.Errorf("alternating tools should emit three M6, got %d", got)
	}
}

// TestEstimateChargesOneToolChangePerSwap checks the cycle-time estimate charges the tool-change
// allowance only on actual swaps: an identical two-op program costs exactly one tool change less
// when both ops share a tool than when the second op switches tools.
func TestEstimateChargesOneToolChangePerSwap(t *testing.T) {
	t1 := ToolController{ToolNumber: 1, VertFeed: 100, HorizFeed: 100}
	t2 := ToolController{ToolNumber: 2, VertFeed: 100, HorizFeed: 100}
	sameTool := []OperationResult{
		{Path: NewJobPath("G1 X10 F100"), Controller: t1},
		{Path: NewJobPath("G1 X20 F100"), Controller: t1},
	}
	diffTool := []OperationResult{
		{Path: NewJobPath("G1 X10 F100"), Controller: t1},
		{Path: NewJobPath("G1 X20 F100"), Controller: t2},
	}
	// The paths are identical, so the only difference is the second op's tool change.
	delta := EstimateMinutes(diffTool) - EstimateMinutes(sameTool)
	if delta < toolChangeSeconds/60-1e-9 || delta > toolChangeSeconds/60+1e-9 {
		t.Errorf("a tool switch should add exactly one tool-change allowance, got a %g min difference", delta)
	}
}
