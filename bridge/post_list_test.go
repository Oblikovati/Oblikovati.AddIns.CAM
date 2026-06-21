// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"strings"
	"testing"
)

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

// TestPostObjectsEstimateComment checks each operation block opens with a cycle-time estimate
// comment naming the operation and a minutes figure scaled by the cut length.
func TestPostObjectsEstimateComment(t *testing.T) {
	tc := ToolController{ToolNumber: 1, VertFeed: 100, HorizFeed: 100}
	short := []OperationResult{{Label: "Tab", Path: NewJobPath("G0 X0", "G1 X10 F100"), Controller: tc}}
	long := []OperationResult{{Label: "Tab", Path: NewJobPath("G0 X0", "G1 X200 F100"), Controller: tc}}

	first := commandNames(lastObj(PostObjects(short)).Path.Commands)[0]
	if !strings.HasPrefix(first, "(Tab: est ") {
		t.Errorf("op block should open with its estimate comment, got %q", first)
	}
	// A longer cut yields a larger estimate figure.
	if minutesOf(t, long) <= minutesOf(t, short) {
		t.Errorf("a longer toolpath should estimate more time: long %g, short %g", minutesOf(t, long), minutesOf(t, short))
	}
}

// minutesOf extracts the "est N.N min" figure from an operation's leading estimate comment.
func minutesOf(t *testing.T, res []OperationResult) float64 {
	t.Helper()
	comment := commandNames(lastObj(PostObjects(res)).Path.Commands)[0]
	var m float64
	if _, err := fmt.Sscanf(comment, "(Tab: est %f min)", &m); err != nil {
		t.Fatalf("could not parse estimate from %q: %v", comment, err)
	}
	return m
}

// TestToolListHeader checks the posted program opens with a setup-sheet listing each distinct tool
// once, in first-use order, with its number, shape, and diameter.
func TestToolListHeader(t *testing.T) {
	res := []OperationResult{
		{Label: "Drill", Path: NewJobPath("G81"), Controller: ToolController{ToolNumber: 2, Tool: ToolBit{ShapeType: "drill", Diameter: 5}}},
		{Label: "Profile", Path: NewJobPath("G1 X1"), Controller: ToolController{ToolNumber: 1, Tool: ToolBit{ShapeType: "endmill", Diameter: 6}}},
		{Label: "Profile2", Path: NewJobPath("G1 X2"), Controller: ToolController{ToolNumber: 1, Tool: ToolBit{ShapeType: "endmill", Diameter: 6}}}, // repeat T1
	}
	objs := PostObjects(res)
	if objs[0].Label != "Tool list" {
		t.Fatalf("first object should be the tool list, got %q", objs[0].Label)
	}
	names := commandNames(objs[0].Path.Commands)
	if len(names) != 2 {
		t.Fatalf("two distinct tools should give two header lines, got %v", names)
	}
	if !strings.Contains(names[0], "T2") || !strings.Contains(names[0], "drill") || !strings.Contains(names[0], "D5.0mm") {
		t.Errorf("first header line should describe T2 drill ⌀5, got %q", names[0])
	}
	if !strings.Contains(names[1], "T1") || !strings.Contains(names[1], "endmill") {
		t.Errorf("second header line should describe T1 end mill, got %q", names[1])
	}
}

// TestPostObjectsOptionalStop checks an operation flagged PauseAfter ends with an M1 optional
// stop, and one not flagged does not.
func TestPostObjectsOptionalStop(t *testing.T) {
	paused := []OperationResult{{Label: "Rough", Path: NewJobPath("G1 X1"), PauseAfter: true}}
	names := commandNames(lastObj(PostObjects(paused)).Path.Commands)
	if names[len(names)-1] != "M1" {
		t.Errorf("a paused op should end with M1, got %v", names)
	}
	plain := []OperationResult{{Label: "Rough", Path: NewJobPath("G1 X1")}}
	for _, n := range commandNames(lastObj(PostObjects(plain)).Path.Commands) {
		if n == "M1" {
			t.Error("an unpaused op should not emit M1")
		}
	}
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
