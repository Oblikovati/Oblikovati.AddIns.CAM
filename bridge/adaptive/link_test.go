// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

package adaptive

import (
	"testing"

	"oblikovati.org/cam/bridge/clipper"
)

// clearADisc seeds rp's cleared area with a generous disc around its entry point so the lead/link
// helpers have somewhere clear to work, and returns that disc's scaled radius.
func clearADisc(t *testing.T, rp *regionProcessor, radius int64) {
	t.Helper()
	disc, err := clipper.Offset(clipper.Paths{{rp.entryPoint}}, clipper.Round, clipper.OpenRound, float64(radius), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	rp.cleared.setClearedPaths(disc)
}

func TestResolveLinkPathDirectWhenClear(t *testing.T) {
	rp, res := seededRegion(t)
	clearADisc(t, rp, 4000)
	// Two points both well inside the cleared disc: the link should resolve directly.
	start := res.entryPoint
	end := clipper.IntPoint{X: res.entryPoint.X + 1500, Y: res.entryPoint.Y}
	path, ok, err := rp.resolveLinkPath(start, end, rp.cleared)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("a link between two points inside the cleared disc should resolve")
	}
	if len(path) < 2 {
		t.Fatalf("resolved link path too short: %v", path)
	}
	if path[0] != start || path[len(path)-1] != end {
		t.Fatalf("link path endpoints = %v..%v, want %v..%v", path[0], path[len(path)-1], start, end)
	}
}

func TestResolveLinkPathFailsThroughUncutStock(t *testing.T) {
	rp, res := seededRegion(t)
	clearADisc(t, rp, 2000)
	// An endpoint far outside the small cleared disc, deep in uncut stock: no keep-tool-down link.
	start := res.entryPoint
	end := clipper.IntPoint{X: res.entryPoint.X + 9000, Y: res.entryPoint.Y + 9000}
	_, ok, err := rp.resolveLinkPath(start, end, rp.cleared)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("a link crossing uncut stock should not resolve to a keep-tool-down path")
	}
}

func TestMakeLeadPathReachesClearedArea(t *testing.T) {
	rp, res := seededRegion(t)
	clearADisc(t, rp, 4000)
	// Start near the edge of the cleared disc heading inward toward the centre beacon; the lead-in
	// travels through cleared stock into the interior and succeeds.
	start := clipper.IntPoint{X: res.entryPoint.X + 3500, Y: res.entryPoint.Y}
	startDir := DoublePoint{X: -1, Y: 0}
	beacon := res.entryPoint
	path, ok, err := rp.makeLeadPath(true, start, startDir, beacon, rp.cleared, rp.toolBoundPaths)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("a lead-in toward the cleared disc centre should succeed")
	}
	if len(path) < 2 || path[0] != start {
		t.Fatalf("lead path should start at the start point: %v", path)
	}
}

func TestMakeLeadPathFailsWithNoClearedArea(t *testing.T) {
	rp, res := seededRegion(t)
	// Empty cleared area → the shrunk acceptable region is empty → failure.
	rp.cleared.setClearedPaths(clipper.Paths{})
	_, ok, err := rp.makeLeadPath(true, res.entryPoint, DoublePoint{X: 1, Y: 0}, res.entryPoint, rp.cleared, rp.toolBoundPaths)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("a lead path with no cleared area should fail")
	}
}
