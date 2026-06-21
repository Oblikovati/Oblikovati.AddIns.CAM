// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

package adaptive

import (
	"testing"

	"oblikovati.org/cam/bridge/clipper"
)

func TestIsClearPathWithinClearedHelix(t *testing.T) {
	rp, res := seededRegion(t)
	// A tiny move at the helix centre stays inside the cleared disc → clear.
	tp := clipper.Path{res.entryPoint, {X: res.entryPoint.X + 10, Y: res.entryPoint.Y}}
	clear, err := rp.isClearPath(tp, rp.cleared, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !clear {
		t.Fatal("a move inside the cleared helix disc should be clear")
	}
}

func TestIsClearPathIntoUncutStock(t *testing.T) {
	rp, res := seededRegion(t)
	// A long move from the helix centre out toward a far corner runs through uncut stock → not clear.
	tp := clipper.Path{res.entryPoint, {X: res.entryPoint.X + 8000, Y: res.entryPoint.Y + 8000}}
	clear, err := rp.isClearPath(tp, rp.cleared, 0)
	if err != nil {
		t.Fatal(err)
	}
	if clear {
		t.Fatal("a move through uncut stock should not be clear")
	}
}

func TestIsAllowedToCutThroughShortMove(t *testing.T) {
	rp, res := seededRegion(t)
	// A sub-step move is treated as insignificant and always allowed.
	near := clipper.IntPoint{X: res.toolPos.X + 1, Y: res.toolPos.Y}
	ok, err := rp.isAllowedToCutThrough(res.toolPos, near, rp.cleared, rp.toolBoundPaths, 1.5, false)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("an insignificant short move should be allowed")
	}
}

func TestIsAllowedToCutThroughRejectsOutsideRegion(t *testing.T) {
	rp, res := seededRegion(t)
	// An endpoint well outside the tool-bound region is not allowed.
	outside := clipper.IntPoint{X: -5000, Y: -5000}
	ok, err := rp.isAllowedToCutThrough(res.toolPos, outside, rp.cleared, rp.toolBoundPaths, 1.5, false)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("a move ending outside the region should be rejected")
	}
}

func TestIsAllowedToCutThroughRejectsOverEngagement(t *testing.T) {
	rp, res := seededRegion(t)
	// A long straight plunge from the helix edge deep into uncut stock over-engages → rejected.
	deep := clipper.IntPoint{X: res.entryPoint.X + 6000, Y: res.entryPoint.Y}
	ok, err := rp.isAllowedToCutThrough(res.entryPoint, deep, rp.cleared, rp.toolBoundPaths, 1.5, false)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("a long over-engaging cut should be rejected")
	}
}
