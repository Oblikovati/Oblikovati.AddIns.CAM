// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"strings"
	"testing"
)

// TestMultiFixtureRepeatsProgram checks that selecting two work coordinate systems posts the
// program in each — more G-code than a single fixture, and the second fixture's offset (G55) is
// present.
func TestMultiFixtureRepeatsProgram(t *testing.T) {
	base, err := NewEngine(&recordingHost{}).RunDrillingJobOnHost(0)
	if err != nil {
		t.Fatalf("single-fixture job: %v", err)
	}
	e := NewEngine(&recordingHost{})
	e.applyPanelEdit("wcs_1", "true") // G54
	e.applyPanelEdit("wcs_2", "true") // G55
	multi, err := e.RunDrillingJobOnHost(0)
	if err != nil {
		t.Fatalf("multi-fixture job: %v", err)
	}
	if multi.GCodeLines <= base.GCodeLines {
		t.Errorf("two fixtures should produce more G-code: %d vs %d lines", multi.GCodeLines, base.GCodeLines)
	}
	if !strings.Contains(multi.GCode, "G55") {
		t.Errorf("second fixture (G55) missing from the multi-fixture program:\n%s", multi.GCode)
	}
}

// TestOrderResultsByMode checks the per-fixture repeat unit: Fixture repeats the whole program,
// Operation one group per operation, Tool one group per tool number.
func TestOrderResultsByMode(t *testing.T) {
	results := []OperationResult{
		{Controller: ToolController{ToolNumber: 1}},
		{Controller: ToolController{ToolNumber: 1}},
		{Controller: ToolController{ToolNumber: 2}},
	}
	if g := orderResults(results, "Fixture"); len(g) != 1 || len(g[0]) != 3 {
		t.Errorf("Fixture grouping = %d groups", len(g))
	}
	if g := orderResults(results, "Operation"); len(g) != 3 {
		t.Errorf("Operation grouping = %d groups, want 3", len(g))
	}
	if g := orderResults(results, "Tool"); len(g) != 2 {
		t.Errorf("Tool grouping = %d groups, want 2", len(g))
	}
}
