// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

package adaptive

import (
	"testing"

	"oblikovati.org/cam/bridge/clipper"
)

func TestAllPointsOutsideStock(t *testing.T) {
	stock := clipper.Paths{{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 100}, {X: 0, Y: 100}}}
	inside := clipper.Path{{X: 40, Y: 40}, {X: 60, Y: 60}}
	if allPointsOutsideStock(inside, stock) {
		t.Fatal("a path through the stock should not be reported as entirely outside")
	}
	outside := clipper.Path{{X: 200, Y: 200}, {X: 300, Y: 300}}
	if !allPointsOutsideStock(outside, stock) {
		t.Fatal("a path clear of the stock should be reported as entirely outside")
	}
}

func TestFinishingToolBoundFollowsPaths(t *testing.T) {
	fin := clipper.Paths{{{X: 0, Y: 0}, {X: 1000, Y: 0}, {X: 1000, Y: 1000}, {X: 0, Y: 1000}}}
	tbp := finishingToolBound(fin)
	if len(tbp) == 0 {
		t.Fatal("finishing tool bound should not be empty for a real finishing path")
	}
}

func TestProcessClearsAndFinishes(t *testing.T) {
	s := newSolver(DefaultConfig())
	if err := s.buildToolGeometry(); err != nil {
		t.Fatal(err)
	}
	bound := clipper.Paths{{{X: 0, Y: 0}, {X: 12000, Y: 0}, {X: 12000, Y: 12000}, {X: 0, Y: 12000}}}
	r := s.toolRadiusScaled
	toolBound := clipper.Paths{{{X: r, Y: r}, {X: 12000 - r, Y: r}, {X: 12000 - r, Y: 12000 - r}, {X: r, Y: 12000 - r}}}
	// Finishing path: the wall contour the tool centre rides (boundary inset by the tool radius).
	finishing, err := clipper.Offset(bound, clipper.Round, clipper.ClosedPolygon, float64(-r), 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	out := &Output{}
	rp, err := newRegionProcessor(s, bound, toolBound, clipper.Paths{}, out)
	if err != nil {
		t.Fatal(err)
	}
	if err := rp.process(bound, finishing); err != nil {
		t.Fatal(err)
	}
	if out.StartPointNotFound {
		t.Fatal("the pocket should have been entered")
	}
	if len(out.AdaptivePaths) == 0 {
		t.Fatal("process should produce toolpaths")
	}
	// The return-motion type is decided at the end of the finishing pass.
	if out.ReturnMotion != MotionLinkClear && out.ReturnMotion != MotionLinkNotClear {
		t.Fatalf("finishing pass should set the return motion, got %v", out.ReturnMotion)
	}
	if frac := clearedFraction(rp); frac < 0.9 {
		t.Fatalf("cleared only %.0f%% including finishing, want >=90%%", frac*100)
	}
}

func TestProcessSkipsFinishingWhenDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FinishingProfile = false
	s := newSolver(cfg)
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
	// No finishing paths needed when finishing is disabled.
	if err := rp.process(bound, nil); err != nil {
		t.Fatal(err)
	}
	if len(out.AdaptivePaths) == 0 {
		t.Fatal("clearing should still produce toolpaths with finishing disabled")
	}
}
