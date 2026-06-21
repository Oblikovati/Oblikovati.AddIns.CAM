// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

package adaptive

import (
	"testing"

	"oblikovati.org/cam/bridge/clipper"
)

func TestRotateToClosest(t *testing.T) {
	ring := clipper.Path{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 100}, {X: 0, Y: 100}}
	prev := clipper.IntPoint{X: 95, Y: 105}
	got := rotateToClosest(ring, &prev)
	if got[0] != (clipper.IntPoint{X: 100, Y: 100}) {
		t.Fatalf("rotated ring should start at the closest vertex (100,100), got %v", got[0])
	}
	if len(got) != len(ring) {
		t.Fatalf("rotation changed the vertex count: %d vs %d", len(got), len(ring))
	}
	// No previous position → unchanged.
	if same := rotateToClosest(ring, nil); same[0] != ring[0] {
		t.Fatal("rotateToClosest(nil) should leave the ring unchanged")
	}
}

func TestLinkCostPenalisesRetraction(t *testing.T) {
	prev := clipper.IntPoint{X: 0, Y: 0}
	// A clear link travels 10mm; a not-clear link travels the same but carries the retraction penalty.
	clear := []TPath{{Motion: MotionLinkClear, Pts: DPath{{X: 0, Y: 0}, {X: 10, Y: 0}}}}
	notClear := []TPath{{Motion: MotionLinkNotClear, Pts: DPath{{X: 0, Y: 0}, {X: 10, Y: 0}}}}
	cClear := linkCostMM(&prev, clear, 1)
	cNot := linkCostMM(&prev, notClear, 1)
	if cNot <= cClear {
		t.Fatalf("a retracting link (%g) should cost more than a clear one (%g)", cNot, cClear)
	}
	if cNot-cClear < 9000 {
		t.Fatalf("retraction penalty too small: %g", cNot-cClear)
	}
}

func TestGetEngagePointFindsFirstEngagement(t *testing.T) {
	rp, _ := seededRegion(t)
	clearADisc(t, rp, 4000)
	tbpMinus, err := clipper.Offset(rp.toolBoundPaths, clipper.Round, clipper.ClosedPolygon, -2, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	res, err := rp.getEngagePoint(nil, rp.toolBoundPaths, tbpMinus)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("a cleared disc inside the region should offer an engage point")
	}
	if len(res.link) == 0 {
		t.Fatal("the engage result should carry a lead-in link")
	}
	if !isPointWithinCutRegion(rp.toolBoundPaths, res.pos) {
		t.Fatalf("engage point %v fell outside the tool-bound region", res.pos)
	}
}

func TestGetEngagePointNoneWithoutClearedArea(t *testing.T) {
	rp, _ := seededRegion(t)
	// No cleared area at all (before any helix entry): there is no border band to engage on, so the
	// caller must fall back to a helix plunge.
	rp.cleared.setClearedPaths(clipper.Paths{})
	tbpMinus, err := clipper.Offset(rp.toolBoundPaths, clipper.Round, clipper.ClosedPolygon, -2, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	res, err := rp.getEngagePoint(nil, rp.toolBoundPaths, tbpMinus)
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		t.Fatalf("an empty cleared area should offer no engage point, got %v", res.pos)
	}
}
