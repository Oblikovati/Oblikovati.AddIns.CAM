// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"testing"
)

// TestRestClearsOnlyWallBand checks rest machining cuts only the band the previous tool missed:
// its outermost ring matches the pocket's, but it stops short of the centre, so it lays down
// strictly fewer rings than a full pocket of the same tool.
func TestRestClearsOnlyWallBand(t *testing.T) {
	rest, err := GenerateRest(square(20), []float64{0}, testFeeds, RestParams{ToolRadius: 1, PrevRadius: 3, StepOver: 0.5, Climb: true})
	if err != nil {
		t.Fatalf("GenerateRest: %v", err)
	}
	// outermost ring = boundary offset in by the current radius → 18×18 = 324, same as a pocket's.
	if a := cutPolygon(rest).Area(); !approx(a, 324) {
		t.Errorf("rest outer ring area = %g, want 324 (18×18)", a)
	}
	// band [1,3) at a 1mm step → rings at d=1,2 → 2 rings; a full pocket would have ~9.
	restRingCount := countPlunges(rest)
	pocket, err := GeneratePocket(square(20), []float64{0}, testFeeds, PocketParams{ToolRadius: 1, StepOver: 0.5, Climb: true})
	if err != nil {
		t.Fatalf("GeneratePocket: %v", err)
	}
	if restRingCount >= countPlunges(pocket) {
		t.Errorf("rest should cut fewer rings than a full pocket: rest=%d pocket=%d", restRingCount, countPlunges(pocket))
	}
	if restRingCount == 0 {
		t.Error("rest produced no wall-band rings")
	}
}

// TestRestErrors covers the degenerate tool relationships.
func TestRestErrors(t *testing.T) {
	if _, err := GenerateRest(square(20), []float64{0}, testFeeds, RestParams{ToolRadius: 0, PrevRadius: 3}); err == nil {
		t.Error("a zero tool radius must error")
	}
	if _, err := GenerateRest(square(20), []float64{0}, testFeeds, RestParams{ToolRadius: 3, PrevRadius: 2}); err == nil {
		t.Error("a previous tool no larger than the current one must error")
	}
	if _, err := GenerateRest(square(2), []float64{0}, testFeeds, RestParams{ToolRadius: 2, PrevRadius: 5}); err == nil {
		t.Error("a tool too large to enter the region must error")
	}
}
