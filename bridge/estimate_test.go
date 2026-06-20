// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"strings"
	"testing"
)

// TestEstimateMinutes sums feed time plus the per-operation tool-change allowance.
func TestEstimateMinutes(t *testing.T) {
	res := []OperationResult{{
		Controller: ToolController{HorizFeed: 100, HorizRapid: 1000},
		// 100 mm + 100 mm of cutting at F200 = 1.0 min; the Z rapid does not move.
		Path: NewJobPath("G0 X0 Y0 Z10", "G1 X100 Y0 F200", "G1 X100 Y100 F200", "G0 Z10"),
	}}
	got := EstimateMinutes(res)
	want := 1.0 + toolChangeSeconds/60 // cut time + one tool change
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("EstimateMinutes = %g, want %g", got, want)
	}
}

// TestMoveRate picks the rapid rate, the move feed, then the controller feed.
func TestMoveRate(t *testing.T) {
	tc := ToolController{HorizFeed: 250, HorizRapid: 1200}
	if r := moveRate(NewJobPath("G0 X1").Commands[0], tc); r != 1200 {
		t.Errorf("rapid rate = %g, want 1200", r)
	}
	if r := moveRate(NewJobPath("G1 X1 F90").Commands[0], tc); r != 90 {
		t.Errorf("feed move rate = %g, want its F90", r)
	}
	if r := moveRate(NewJobPath("G1 X1").Commands[0], tc); r != 250 {
		t.Errorf("feed move without F = %g, want the controller feed 250", r)
	}
}

// TestEstimateReachesSummary confirms a generated job reports an estimate.
func TestEstimateReachesSummary(t *testing.T) {
	res, err := NewEngine(&recordingHost{}).RunProfileJobOnHost(0)
	if err != nil {
		t.Fatalf("RunProfileJobOnHost: %v", err)
	}
	if res.EstimatedMinutes <= 0 {
		t.Errorf("profile job should report a positive estimate, got %g", res.EstimatedMinutes)
	}
	if !strings.Contains(res.Summary, "min.") {
		t.Errorf("summary should mention the estimated minutes: %q", res.Summary)
	}
}
