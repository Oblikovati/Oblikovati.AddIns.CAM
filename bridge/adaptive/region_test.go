// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

package adaptive

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/clipper"
)

// seededRegion builds a solver and a large square region, runs the real entry search to seed the
// cleared area with the helix disc, and returns a regionProcessor ready to step. It is the live
// fixture for the engagement-loop tests (cgo only — it drives the clipping engine).
func seededRegion(t *testing.T) (*regionProcessor, entryResult) {
	t.Helper()
	s := newSolver(DefaultConfig())
	if err := s.buildToolGeometry(); err != nil {
		t.Fatal(err)
	}
	bound := clipper.Paths{{{X: 0, Y: 0}, {X: 20000, Y: 0}, {X: 20000, Y: 20000}, {X: 0, Y: 20000}}}
	toolBound := clipper.Paths{{{X: 1200, Y: 1200}, {X: 18800, Y: 1200}, {X: 18800, Y: 18800}, {X: 1200, Y: 18800}}}
	cleared := newClearedArea(s.toolRadiusScaled)
	res, err := s.findEntryPoint(toolBound, bound, cleared)
	if err != nil {
		t.Fatal(err)
	}
	if !res.found {
		t.Fatal("entry should be found in a large region")
	}
	rp := &regionProcessor{
		s:              s,
		toolBoundPaths: toolBound,
		boundPaths:     bound,
		cleared:        cleared,
		entryPoint:     res.entryPoint,
		angle:          math.Pi,
		output:         &Output{},
	}
	return rp, res
}

func TestInitToolDirFindsADirection(t *testing.T) {
	rp, res := seededRegion(t)
	dir, found, err := rp.initToolDir(res.toolPos, res.toolDir)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("a direction should be found stepping off the seeded helix")
	}
	if l := math.Hypot(dir.X, dir.Y); math.Abs(l-1) > 1e-6 {
		t.Fatalf("returned direction should be a unit vector, |dir| = %g", l)
	}
}

func TestIterateNextStepCutsAreaInsideRegion(t *testing.T) {
	rp, res := seededRegion(t)
	dir, found, err := rp.initToolDir(res.toolPos, res.toolDir)
	if err != nil || !found {
		t.Fatalf("initToolDir failed: found=%v err=%v", found, err)
	}
	out, err := rp.iterateNextStep(res.toolPos, dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if out.area <= 0 {
		t.Fatalf("a step off the helix should cut material, area = %g", out.area)
	}
	if !isPointWithinCutRegion(rp.toolBoundPaths, out.newToolPos) {
		t.Fatalf("stepped tool position %v left the cut region", out.newToolPos)
	}
	// The step length should sit in the stable range the schedule clamps to.
	if rp.stepScaled < int64(minStepClipper) || rp.stepScaled > int64(minStepClipper*8) {
		t.Fatalf("step %d outside the stable [%g, %g] range", rp.stepScaled, minStepClipper, minStepClipper*8)
	}
}

func TestIterateNextStepEngagementBounded(t *testing.T) {
	rp, res := seededRegion(t)
	dir, _, err := rp.initToolDir(res.toolPos, res.toolDir)
	if err != nil {
		t.Fatal(err)
	}
	out, err := rp.iterateNextStep(res.toolPos, dir, false)
	if err != nil {
		t.Fatal(err)
	}
	// The solver must never grossly overcut: the area-per-distance is capped at twice the optimal
	// (beyond that it flags failure), so a non-failed step stays at or under that bound.
	if !out.failed {
		areaPD := out.area / float64(rp.stepScaled)
		if areaPD > 2*rp.s.optimalCutAreaPD+1e-6 {
			t.Fatalf("engagement areaPD %g exceeds 2x optimal %g on a non-failed step", areaPD, rp.s.optimalCutAreaPD)
		}
	}
}
