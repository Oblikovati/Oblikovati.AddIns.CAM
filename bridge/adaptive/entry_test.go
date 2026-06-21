// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import (
	"testing"

	"oblikovati.org/cam/bridge/clipper"
)

func TestFindEntryPointFitsLargeRegion(t *testing.T) {
	s := newSolver(DefaultConfig()) // toolRadiusScaled 1200, helix 150..1200
	// A 20000 x 20000 region with a tool-centre bound inset by the tool radius: the entry helix
	// fits easily near the centre.
	bound := clipper.Paths{{{X: 0, Y: 0}, {X: 20000, Y: 0}, {X: 20000, Y: 20000}, {X: 0, Y: 20000}}}
	toolBound := clipper.Paths{{{X: 1200, Y: 1200}, {X: 18800, Y: 1200}, {X: 18800, Y: 18800}, {X: 1200, Y: 18800}}}
	cleared := newClearedArea(s.toolRadiusScaled)

	res, err := s.findEntryPoint(toolBound, bound, cleared)
	if !engineAvailable() {
		if err == nil {
			t.Fatal("findEntryPoint should error without the cgo engine")
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if !res.found {
		t.Fatal("entry helix should fit in a large open region")
	}
	if res.helixRadiusScaled < s.helixRampMinRadiusScaled || res.helixRadiusScaled > s.helixRampMaxRadiusScaled {
		t.Fatalf("helix radius %d out of [%d,%d]", res.helixRadiusScaled, s.helixRampMinRadiusScaled, s.helixRampMaxRadiusScaled)
	}
	// Entry near the region centre (10000,10000); tool starts a helix-radius below it, heading +X.
	if abs64(res.entryPoint.X-10000) > 2000 || abs64(res.entryPoint.Y-10000) > 2000 {
		t.Fatalf("entry point %v not near centre (10000,10000)", res.entryPoint)
	}
	if res.toolPos != (clipper.IntPoint{X: res.entryPoint.X, Y: res.entryPoint.Y - res.helixRadiusScaled}) {
		t.Fatalf("toolPos %v should sit a helix radius below the entry", res.toolPos)
	}
	if res.toolDir != (DoublePoint{X: 1, Y: 0}) {
		t.Fatalf("toolDir = %v, want (1,0)", res.toolDir)
	}
	// The helix disc should have seeded the cleared area.
	if len(cleared.cleared()) == 0 {
		t.Fatal("entry should seed the cleared area with the helix disc")
	}
}

func TestFindEntryPointTooTight(t *testing.T) {
	s := newSolver(DefaultConfig())
	if !engineAvailable() {
		t.Skip("requires the cgo engine")
	}
	// A 2600-wide region: the tool-centre bound is a sliver (200 wide) and even the minimum helix
	// (radius 150 + tool 1200 = 1350) cannot fit without crossing the boundary.
	bound := clipper.Paths{{{X: 0, Y: 0}, {X: 2600, Y: 0}, {X: 2600, Y: 2600}, {X: 0, Y: 2600}}}
	toolBound := clipper.Paths{{{X: 1200, Y: 1200}, {X: 1400, Y: 1200}, {X: 1400, Y: 1400}, {X: 1200, Y: 1400}}}
	res, err := s.findEntryPoint(toolBound, bound, newClearedArea(s.toolRadiusScaled))
	if err != nil {
		t.Fatal(err)
	}
	if res.found {
		t.Fatal("no helix should fit in a region too tight for the minimum ramp")
	}
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
