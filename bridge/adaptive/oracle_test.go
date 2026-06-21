// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

package adaptive

import (
	"math"
	"testing"
)

// These tests mirror the upstream Adaptive2d unit tests (CAMTests/TestPathAdaptive.py:
// testClearInside / testClearInsideWithRestMachining / testClearOutside) exactly: the same
// rectangle geometry and tool, and the same closed-form expected cleared area and tolerance. The
// upstream test asserts the C++ solver's total cleared area lands within half a tool-corner of the
// analytic value; this asserts our port lands within the same band — i.e. it clears what the
// original clears.

// oracleConfig is the default Adaptive2d configuration the upstream tests use.
func oracleConfig() Config {
	return Config{
		ToolDiameter:          5.0,
		StepOverFactor:        0.20,
		Tolerance:             0.1,
		ForceInsideOut:        false,
		FinishingProfile:      true,
		KeepToolDownDistRatio: 3.0,
		OpType:                ClearingInside,
	}
}

// rectDPath returns a rectangle contour (mm).
func rectDPath(x0, y0, x1, y1 float64) DPath {
	return DPath{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}
}

// cornerUnclearable is the area a round tool leaves in a single 90° corner: r²(1 − π/4).
func cornerUnclearable(toolDiameter float64) float64 {
	r := toolDiameter / 2
	return r * r * (1 - math.Pi/4)
}

func totalCleared(outputs []Output) float64 {
	t := 0.0
	for _, o := range outputs {
		t += o.ClearedArea
	}
	return t
}

// 50×50 stock, 40×40 pocket, clearing inside: cleared ≈ 1600 − 4 corners.
func TestOracleClearInside(t *testing.T) {
	stock := []DPath{rectDPath(0, 0, 50, 50)}
	pocket := []DPath{rectDPath(5, 5, 45, 45)}
	outputs, err := Execute(oracleConfig(), stock, pocket, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertNoErrorFlags(t, outputs)

	corner := cornerUnclearable(5.0)
	expected := 40.0*40.0 - 4*corner
	delta := corner / 2
	if got := totalCleared(outputs); math.Abs(got-expected) > delta {
		t.Fatalf("cleared inside = %.3f, want %.3f ± %.3f", got, expected, delta)
	}
}

// 50×50 stock, 40×40 pocket, two opposite quadrants pre-cleared (rest machining).
func TestOracleClearInsideRestMachining(t *testing.T) {
	stock := []DPath{rectDPath(0, 0, 50, 50)}
	pocket := []DPath{rectDPath(5, 5, 45, 45)}
	cleared := []DPath{
		rectDPath(0, 0, 25, 25),   // bottom-left quadrant
		rectDPath(25, 25, 50, 50), // top-right quadrant
	}
	outputs, err := Execute(oracleConfig(), stock, pocket, cleared)
	if err != nil {
		t.Fatal(err)
	}
	assertNoErrorFlags(t, outputs)

	corner := cornerUnclearable(5.0)
	// Two of the four corners sit in the pre-cleared quadrants.
	// Overlap of the pocket [5,45]² with each quadrant is a 20×20 square.
	overlap := 2 * (20.0 * 20.0)
	expected := 40.0*40.0 - 2*corner - overlap
	delta := corner / 2
	if got := totalCleared(outputs); math.Abs(got-expected) > delta {
		t.Fatalf("rest-machining cleared = %.3f, want %.3f ± %.3f", got, expected, delta)
	}
}

// TestOracleClearOutside mirrors the upstream testClearOutside: 50×50 stock, 40×40 part centred,
// clearing OUTSIDE the part with forceInsideOut=false. The stock-overshoot path (Execute step 5)
// bounds the cut to a frame around the stock, so the cleared area is stock − part = 2500 − 1600,
// within half a tool corner.
func TestOracleClearOutside(t *testing.T) {
	cfg := oracleConfig()
	cfg.OpType = ClearingOutside
	stock := []DPath{rectDPath(0, 0, 50, 50)}
	part := []DPath{rectDPath(5, 5, 45, 45)}
	outputs, err := Execute(cfg, stock, part, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertNoErrorFlags(t, outputs)

	expected := 50.0*50.0 - 40.0*40.0
	delta := cornerUnclearable(5.0) / 2
	if got := totalCleared(outputs); math.Abs(got-expected) > delta {
		t.Fatalf("cleared outside = %.3f, want %.3f ± %.3f", got, expected, delta)
	}
}

// profilingConfig is the upstream profiling configuration (stepOverFactor 0.5, the rest as the
// clearing oracle).
func profilingConfig(op OperationType) Config {
	cfg := oracleConfig()
	cfg.OpType = op
	cfg.StepOverFactor = 0.5
	return cfg
}

// TestOracleProfilingInside mirrors the upstream testProfilingInside: profiling INSIDE a 40×40 path
// (in 50×50 stock) clears a band 2–3 tool diameters deep along the inside of the path wall. The
// cleared area must fall between (outer − inner@3⌀) and (outer − inner@2⌀). Exact port of the
// upstream area-band assertion (no magic numbers — the same formula).
func TestOracleProfilingInside(t *testing.T) {
	stock := []DPath{rectDPath(0, 0, 50, 50)}
	path := []DPath{rectDPath(5, 5, 45, 45)} // 40×40 centred
	outputs, err := Execute(profilingConfig(ProfilingInside), stock, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertNoErrorFlags(t, outputs)

	const d = 5.0 // tool diameter
	outerArea := 40.0 * 40.0
	innerAreaMax := bandInnerArea(40.0, 2*d) // 2 diameters deep leaves the largest island
	innerAreaMin := bandInnerArea(40.0, 3*d) // 3 diameters deep leaves the smallest
	minArea := outerArea - innerAreaMax
	maxArea := outerArea - innerAreaMin

	if got := totalCleared(outputs); got < minArea || got > maxArea {
		t.Fatalf("profiling-inside cleared = %.3f, want in [%.3f, %.3f] (2–3 tool diameters)", got, minArea, maxArea)
	}
}

// TestOracleProfilingOutside mirrors the upstream testProfilingOutside: profiling OUTSIDE a 15×15
// path (in 50×50 stock) clears a band 2–3 tool diameters wide around the path, bounded to the stock.
// The band math accounts for the rounded outer corners the round tool leaves and the inner corners
// it cannot reach. Exact port of the upstream formula.
func TestOracleProfilingOutside(t *testing.T) {
	stock := []DPath{rectDPath(0, 0, 50, 50)}
	path := []DPath{rectDPath(17.5, 17.5, 32.5, 32.5)} // 15×15 centred
	outputs, err := Execute(profilingConfig(ProfilingOutside), stock, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertNoErrorFlags(t, outputs)

	const d, stockSide = 5.0, 50.0
	innerArea := 15.0 * 15.0

	outerAreaMin := bandOuterArea(15.0, 2*d, stockSide)
	outerAreaMax := bandOuterArea(15.0, 3*d, stockSide)
	roundCorner := func(off float64) float64 { return 4 * off * off * (1 - math.Pi/4) }
	innerUnclearable := 4 * cornerUnclearable(d)

	minArea := outerAreaMin - innerArea - roundCorner(2*d) - innerUnclearable
	maxArea := outerAreaMax - innerArea - roundCorner(3*d)

	if got := totalCleared(outputs); got < minArea || got > maxArea {
		t.Fatalf("profiling-outside cleared = %.3f, want in [%.3f, %.3f] (2–3 tool diameters)", got, minArea, maxArea)
	}
}

// bandInnerArea is the area of the square island left inside a side-length square when a band of the
// given one-sided offset is taken off each edge; zero once the offsets meet.
func bandInnerArea(side, offset float64) float64 {
	inner := side - 2*offset
	if inner <= 0 {
		return 0
	}
	return inner * inner
}

// bandOuterArea is the area of the square reached offsetting a side-length square outward on each
// edge, clamped to the stock side.
func bandOuterArea(side, offset, stockSide float64) float64 {
	outer := math.Min(side+2*offset, stockSide)
	return outer * outer
}

func assertNoErrorFlags(t *testing.T, outputs []Output) {
	t.Helper()
	if len(outputs) == 0 {
		t.Fatal("Adaptive2d should return at least one result")
	}
	for i, o := range outputs {
		if o.StartPointNotFound {
			t.Fatalf("region %d: start point not found", i)
		}
		if o.LeadPathFailed {
			t.Fatalf("region %d: lead path failed", i)
		}
		if o.TooManyFailedEngagements {
			t.Fatalf("region %d: too many failed engagements", i)
		}
		if o.UnclearedAreaRemains {
			t.Fatalf("region %d: uncleared area remains", i)
		}
		if o.FinishingLeadInFailed {
			t.Fatalf("region %d: finishing lead-in failed", i)
		}
	}
}
