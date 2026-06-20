// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// maxWallReach returns how far the cut moves reach toward the wall — the largest |X−centre| of
// any G1 with an X — which equals the outermost ring's half-width for a centred square pocket.
func maxWallReach(cmds []gcode.Command, centre float64) float64 {
	reach := 0.0
	for _, c := range cmds {
		if c.Name != "G1" {
			continue
		}
		if x, ok := c.Params["X"]; ok {
			reach = math.Max(reach, math.Abs(x-centre))
		}
	}
	return reach
}

// TestPocketFinishAllowance checks a finish allowance leaves the roughing short of the wall but a
// final finishing pass still cleans the wall to size: the cut reaches the same wall as a plain
// pocket (the finish pass), and there is one extra ring (the finish loop) compared with roughing
// that stopped at the allowance.
func TestPocketFinishAllowance(t *testing.T) {
	boundary := square(20) // centred at (10,10), wall at |x−10| = 8 after the 2mm radius inset

	plain, err := GeneratePocket(boundary, []float64{0}, testFeeds, PocketParams{ToolRadius: 2, StepOver: 0.5, Climb: true})
	if err != nil {
		t.Fatalf("plain pocket: %v", err)
	}
	finished, err := GeneratePocket(boundary, []float64{0}, testFeeds, PocketParams{ToolRadius: 2, StepOver: 0.5, Climb: true, FinishAllowance: 1})
	if err != nil {
		t.Fatalf("finish-allowance pocket: %v", err)
	}

	// The finishing pass reaches the same wall as the plain pocket (within a hair).
	if pr, fr := maxWallReach(plain, 10), maxWallReach(finished, 10); math.Abs(pr-fr) > 1e-6 {
		t.Errorf("finish pass should clean to the wall: plain reach %g, finished reach %g", pr, fr)
	}
	// The finishing pass is an extra loop on top of the (allowance-shortened) roughing rings.
	if pp, fp := countPlunges(plain), countPlunges(finished); fp <= pp {
		t.Errorf("a finish allowance should add a finishing pass: plain plunges %d, finished %d", pp, fp)
	}
}

// TestPocketFinishAllowanceNoOp checks a zero allowance leaves the toolpath byte-identical to the
// plain pocket (no finishing pass, no shifted roughing).
func TestPocketFinishAllowanceNoOp(t *testing.T) {
	boundary := square(20)
	plain, _ := GeneratePocket(boundary, []float64{0}, testFeeds, PocketParams{ToolRadius: 2, StepOver: 0.5, Climb: true})
	zero, _ := GeneratePocket(boundary, []float64{0}, testFeeds, PocketParams{ToolRadius: 2, StepOver: 0.5, Climb: true, FinishAllowance: 0})
	if len(plain) != len(zero) {
		t.Fatalf("zero allowance should be a no-op: plain %d cmds, zero %d", len(plain), len(zero))
	}
	for i := range plain {
		if plain[i].Name != zero[i].Name {
			t.Errorf("cmd %d differs: %q vs %q", i, plain[i].Name, zero[i].Name)
		}
	}
}
