// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

package adaptive

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/clipper"
)

// clearedFraction returns the fraction of the tool-bound area the processor managed to clear.
func clearedFraction(rp *regionProcessor) float64 {
	boundArea := 0.0
	for _, p := range rp.toolBoundPaths {
		boundArea += math.Abs(clipper.Area(p))
	}
	clearedArea := 0.0
	for _, p := range rp.cleared.cleared() {
		clearedArea += clipper.Area(p)
	}
	if boundArea == 0 {
		return 0
	}
	return clearedArea / boundArea
}

func TestClearRegionClearsASquarePocket(t *testing.T) {
	s := newSolver(DefaultConfig())
	if err := s.buildToolGeometry(); err != nil {
		t.Fatal(err)
	}
	// A ~25mm square pocket (scaled): boundary, and the tool-centre bound inset by the tool radius.
	bound := clipper.Paths{{{X: 0, Y: 0}, {X: 12000, Y: 0}, {X: 12000, Y: 12000}, {X: 0, Y: 12000}}}
	r := s.toolRadiusScaled
	toolBound := clipper.Paths{{{X: r, Y: r}, {X: 12000 - r, Y: r}, {X: 12000 - r, Y: 12000 - r}, {X: r, Y: 12000 - r}}}

	out := &Output{}
	rp, err := newRegionProcessor(s, bound, toolBound, clipper.Paths{}, out)
	if err != nil {
		t.Fatal(err)
	}
	if err := rp.clearRegion(); err != nil {
		t.Fatal(err)
	}

	if out.StartPointNotFound {
		t.Fatal("a square pocket should have a valid entry point")
	}
	if len(out.AdaptivePaths) == 0 {
		t.Fatal("clearing should produce toolpaths")
	}
	// The adaptive clearing should reach almost all of the inset region.
	if frac := clearedFraction(rp); frac < 0.9 {
		t.Fatalf("cleared only %.0f%% of the region, want >=90%%", frac*100)
	}
	// At least one engaged cutting move must be present.
	hasCut := false
	for _, tp := range out.AdaptivePaths {
		if tp.Motion == MotionCutting && len(tp.Pts) >= 2 {
			hasCut = true
			break
		}
	}
	if !hasCut {
		t.Fatal("expected at least one engaged cutting toolpath")
	}
}

func TestClearRegionStartPointWithinBound(t *testing.T) {
	s := newSolver(DefaultConfig())
	if err := s.buildToolGeometry(); err != nil {
		t.Fatal(err)
	}
	bound := clipper.Paths{{{X: 0, Y: 0}, {X: 10000, Y: 0}, {X: 10000, Y: 10000}, {X: 0, Y: 10000}}}
	r := s.toolRadiusScaled
	toolBound := clipper.Paths{{{X: r, Y: r}, {X: 10000 - r, Y: r}, {X: 10000 - r, Y: 10000 - r}, {X: r, Y: 10000 - r}}}
	out := &Output{}
	rp, err := newRegionProcessor(s, bound, toolBound, clipper.Paths{}, out)
	if err != nil {
		t.Fatal(err)
	}
	if err := rp.clearRegion(); err != nil {
		t.Fatal(err)
	}
	// The recorded helix centre / start point must lie inside the region (in mm).
	sf := float64(s.scaleFactor)
	start := clipper.IntPoint{X: int64(out.StartPoint.X * sf), Y: int64(out.StartPoint.Y * sf)}
	if !isPointWithinCutRegion(toolBound, start) {
		t.Fatalf("start point %v is outside the tool-bound region", out.StartPoint)
	}
}
