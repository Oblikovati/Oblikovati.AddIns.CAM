// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

package bridge

import (
	"testing"

	"oblikovati.org/cam/bridge/geom2d"
)

// TestAdaptiveOpSolverEngaged checks that with the cgo clearing engine present, an adaptive op
// produces a clearing toolpath that stays down (few plunges relative to many cuts) and enters
// inside the boundary — the faithful solver path, not the spiral fallback.
func TestAdaptiveOpSolverEngaged(t *testing.T) {
	op := &AdaptiveOp{
		OpBase:   OpBase{OpLabel: "Adaptive", IsActive: true, ClearanceHeight: 15, SafeHeight: 2, StartDepth: 0, FinalDepth: -2},
		StepOver: 0.1,
		Boundary: squarePoly(30),
	}
	path, err := op.Execute(millJob(6))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	plunges, cuts := 0, 0
	for _, c := range path.Commands {
		if c.Name != "G1" {
			continue
		}
		if _, hasZ := c.Params["Z"]; hasZ {
			plunges++
		}
		if _, hasX := c.Params["X"]; hasX {
			cuts++
		}
	}
	if cuts < 20 {
		t.Fatalf("the solver should lay down many low-engagement cuts, got %d", cuts)
	}
	// Stay-down clearing: far fewer plunges than cuts (links keep the tool down where they can).
	if plunges == 0 || plunges*4 > cuts {
		t.Fatalf("expected stay-down clearing (few plunges), got %d plunges for %d cuts", plunges, cuts)
	}
}

// TestAdaptiveOpSolverRoutesAroundIsland checks that an island is left standing: no cutting move
// passes through the island's interior.
func TestAdaptiveOpSolverRoutesAroundIsland(t *testing.T) {
	island := geom2d.Polygon{{X: 12, Y: 12}, {X: 18, Y: 12}, {X: 18, Y: 18}, {X: 12, Y: 18}}
	op := &AdaptiveOp{
		OpBase:   OpBase{OpLabel: "Adaptive", IsActive: true, ClearanceHeight: 15, SafeHeight: 2, StartDepth: 0, FinalDepth: -1},
		StepOver: 0.1,
		Boundary: squarePoly(30),
		Islands:  []geom2d.Polygon{island},
	}
	path, err := op.Execute(millJob(4))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !hasCutMove(path) {
		t.Fatal("clearing around an island should still cut")
	}
	// The tool centre keeps a radius away from a standing island, so no cut should reach its centre.
	for _, c := range path.Commands {
		x, okX := c.Params["X"]
		y, okY := c.Params["Y"]
		if c.Name == "G1" && okX && okY {
			if x > 14 && x < 16 && y > 14 && y < 16 {
				t.Fatalf("a cutting move (%g,%g) entered the standing island centre", x, y)
			}
		}
	}
}

// TestAdaptiveOpSolverTooSmallErrors checks the faithful path errors for a region the tool cannot
// enter (matching the spiral fallback's behaviour).
func TestAdaptiveOpSolverTooSmallErrors(t *testing.T) {
	op := &AdaptiveOp{OpBase: OpBase{OpLabel: "A", IsActive: true}, Boundary: squarePoly(2)}
	if _, err := op.Execute(millJob(6)); err == nil {
		t.Fatal("a region smaller than the tool must error under the solver too")
	}
}
