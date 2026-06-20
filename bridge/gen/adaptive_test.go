// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// countCutMoves counts the horizontal cutting moves (G1 carrying X/Y) in a command list.
func countCutMoves(cmds []gcode.Command) int {
	n := 0
	for _, c := range cmds {
		if _, hasX := c.Params["X"]; c.Name == "G1" && hasX {
			n++
		}
	}
	return n
}

// TestAdaptiveStaysDown checks the adaptive spiral plunges only once per depth level (it links
// the rings without retracting between them) — unlike the offset-pattern pocket, which plunges
// once per ring.
func TestAdaptiveStaysDown(t *testing.T) {
	levels := []float64{-2, -4} // two depth levels
	cmds, err := GenerateAdaptive(square(20), levels, testFeeds, AdaptiveParams{ToolRadius: 2, StepOver: 0.2, Climb: true})
	if err != nil {
		t.Fatalf("GenerateAdaptive: %v", err)
	}
	if got := countPlunges(cmds); got != len(levels) {
		t.Errorf("adaptive plunges = %d, want %d (one stay-down spiral per level)", got, len(levels))
	}

	// the same region+tool cleared as a pocket plunges once per ring → strictly more plunges.
	pocket, err := GeneratePocket(square(20), levels, testFeeds, PocketParams{ToolRadius: 2, StepOver: 0.2, Climb: true})
	if err != nil {
		t.Fatalf("GeneratePocket: %v", err)
	}
	if countPlunges(pocket) <= countPlunges(cmds) {
		t.Errorf("pocket should plunge more than adaptive: pocket=%d adaptive=%d", countPlunges(pocket), countPlunges(cmds))
	}
}

// TestAdaptiveLowStepOver checks the default (small) step-over lays down many more passes than a
// coarse pocket would — the low-engagement HSM signature.
func TestAdaptiveLowStepOver(t *testing.T) {
	fine, err := GenerateAdaptive(square(20), []float64{0}, testFeeds, AdaptiveParams{ToolRadius: 2, Climb: true})
	if err != nil {
		t.Fatalf("GenerateAdaptive: %v", err)
	}
	coarse, err := GeneratePocket(square(20), []float64{0}, testFeeds, PocketParams{ToolRadius: 2, StepOver: 0.5, Climb: true})
	if err != nil {
		t.Fatalf("GeneratePocket: %v", err)
	}
	if countCutMoves(fine) <= countCutMoves(coarse) {
		t.Errorf("adaptive (default 0.1 step-over) should cut more passes than a 0.5 pocket: adaptive=%d pocket=%d",
			countCutMoves(fine), countCutMoves(coarse))
	}
}

// TestAdaptiveErrors covers the degenerate tool/region cases.
func TestAdaptiveErrors(t *testing.T) {
	if _, err := GenerateAdaptive(square(20), []float64{0}, testFeeds, AdaptiveParams{ToolRadius: 0}); err == nil {
		t.Error("a zero tool radius must error")
	}
	if _, err := GenerateAdaptive(square(2), []float64{0}, testFeeds, AdaptiveParams{ToolRadius: 2}); err == nil {
		t.Error("a tool too large for the region must error")
	}
}
