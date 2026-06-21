// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

package adaptive

import (
	"testing"

	"oblikovati.org/cam/bridge/clipper"
)

func TestFindLinkPathFirstEngagementLeadInOnly(t *testing.T) {
	rp, res := seededRegion(t)
	clearADisc(t, rp, 4000)
	// No previous position → only a lead-in is produced (no linking move).
	start := clipper.IntPoint{X: res.entryPoint.X + 3000, Y: res.entryPoint.Y}
	out, ok, err := rp.findLinkPath(nil, start, DoublePoint{X: 0, Y: 1}, rp.cleared, rp.toolBoundPaths)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("a lead-in inside the cleared disc should be found")
	}
	if len(out) != 1 {
		t.Fatalf("first engagement should yield exactly one (lead-in) path, got %d", len(out))
	}
	if out[0].Motion != MotionCutting {
		t.Fatalf("lead-in motion = %v, want cutting", out[0].Motion)
	}
	if len(out[0].Pts) < 2 {
		t.Fatalf("lead-in too short: %v", out[0].Pts)
	}
}

func TestFindLinkPathWithPrevAddsLink(t *testing.T) {
	rp, res := seededRegion(t)
	clearADisc(t, rp, 4000)
	prev := clipper.IntPoint{X: res.entryPoint.X - 2000, Y: res.entryPoint.Y}
	start := clipper.IntPoint{X: res.entryPoint.X + 2000, Y: res.entryPoint.Y}
	out, ok, err := rp.findLinkPath(&prev, start, DoublePoint{X: 0, Y: 1}, rp.cleared, rp.toolBoundPaths)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("a link between two cleared points should be found")
	}
	// With a previous position the result carries a linking move plus the lead-in.
	if len(out) != 2 {
		t.Fatalf("expected link + lead-in (2 paths), got %d", len(out))
	}
	if out[1].Motion != MotionCutting {
		t.Fatalf("final path should be the cutting lead-in, got %v", out[1].Motion)
	}
}

func TestAppendToolPathEmitsCutAndLeadOut(t *testing.T) {
	rp, res := seededRegion(t)
	clearADisc(t, rp, 4000)
	// A short engaged pass through the cleared interior.
	pass := clipper.Path{
		{X: res.entryPoint.X - 1500, Y: res.entryPoint.Y},
		{X: res.entryPoint.X, Y: res.entryPoint.Y},
		{X: res.entryPoint.X + 1500, Y: res.entryPoint.Y},
	}
	before := len(rp.output.AdaptivePaths)
	pos, dir, ok, err := rp.appendToolPath(pass, nil, rp.cleared, rp.toolBoundPaths)
	if err != nil {
		t.Fatal(err)
	}
	// The cut path must always be emitted, regardless of whether a lead-out was found.
	if len(rp.output.AdaptivePaths) <= before {
		t.Fatal("appendToolPath should append at least the cut path")
	}
	if rp.output.AdaptivePaths[before].Motion != MotionCutting {
		t.Fatalf("first appended path should be the cut, got %v", rp.output.AdaptivePaths[before].Motion)
	}
	if ok {
		// when a lead-out was produced the returned direction is a unit vector
		if l := dir.X*dir.X + dir.Y*dir.Y; l < 0.9 || l > 1.1 {
			t.Fatalf("lead-out direction not unit length: |dir|^2 = %g", l)
		}
		_ = pos
	}
}

func TestAppendToolPathEmptyPassNoOp(t *testing.T) {
	rp, _ := seededRegion(t)
	clearADisc(t, rp, 4000)
	_, _, ok, err := rp.appendToolPath(clipper.Path{}, nil, rp.cleared, rp.toolBoundPaths)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("an empty pass should not yield a lead-out")
	}
}
