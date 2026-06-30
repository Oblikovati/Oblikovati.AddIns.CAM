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

// TestGroupResultsByTool checks results group by tool number, first-seen order.
func TestGroupResultsByTool(t *testing.T) {
	results := []OperationResult{
		{Controller: ToolController{ToolNumber: 1}},
		{Controller: ToolController{ToolNumber: 1}},
		{Controller: ToolController{ToolNumber: 2}},
	}
	groups := groupResultsByTool(results)
	if len(groups) != 2 || len(groups[0].results) != 2 || groups[1].suffix != "T2" {
		t.Errorf("tool grouping = %+v", groups)
	}
}

// TestSplitOutputProducesFilePerFixture checks split output records one program unit per fixture,
// named by its work coordinate system.
func TestSplitOutputProducesFilePerFixture(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.applyPanelEdit("wcs_1", "true") // G54
	e.applyPanelEdit("wcs_2", "true") // G55
	e.applyPanelEdit("split_output", "true")
	if _, err := e.RunDrillingJobOnHost(0); err != nil {
		t.Fatalf("job: %v", err)
	}
	if len(e.lastPrograms) != 2 {
		t.Fatalf("split should record 2 programs, got %d", len(e.lastPrograms))
	}
	if e.lastPrograms[0].Suffix != "G54" || e.lastPrograms[1].Suffix != "G55" {
		t.Errorf("suffixes = %q, %q", e.lastPrograms[0].Suffix, e.lastPrograms[1].Suffix)
	}
}

// TestNoSplitWhenOff checks split units are not recorded with split output off.
func TestNoSplitWhenOff(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.applyPanelEdit("wcs_1", "true")
	e.applyPanelEdit("wcs_2", "true") // multiple fixtures, but split is off
	if _, err := e.RunDrillingJobOnHost(0); err != nil {
		t.Fatalf("job: %v", err)
	}
	if e.lastPrograms != nil {
		t.Errorf("no split units expected with split off, got %d", len(e.lastPrograms))
	}
}
