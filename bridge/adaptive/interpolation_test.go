// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/clipper"
)

func TestInterpolationEmptyAndSingle(t *testing.T) {
	var ip interpolation
	if ip.getPointCount() != 0 {
		t.Fatal("a fresh interpolation should hold no points")
	}
	if ip.interpolateAngle() != interpMinAngle {
		t.Fatal("no samples → min angle")
	}
	ip.addPoint(5, 0.3, clipper.IntPoint{}, false, false)
	if ip.getPointCount() != 1 {
		t.Fatal("one sample expected")
	}
	if ip.interpolateAngle() != interpMaxAngle {
		t.Fatal("one (upper) sample → max angle")
	}
}

func TestInterpolationBracketsZero(t *testing.T) {
	var ip interpolation
	// A lower sample (error -10 at angle -0.5) and an upper sample (+10 at +0.5) bracket zero;
	// the interpolated angle is the midpoint 0.
	ip.addPoint(-10, -0.5, clipper.IntPoint{}, false, false)
	ip.addPoint(10, 0.5, clipper.IntPoint{}, false, false)
	if !ip.bothSides() {
		t.Fatal("the two samples should bracket zero error")
	}
	if got := ip.interpolateAngle(); math.Abs(got) > 1e-9 {
		t.Fatalf("interpolated angle = %g, want 0", got)
	}
}

func TestInterpolationOrderIndependent(t *testing.T) {
	// Adding the samples in the opposite order must swap them into (min,max) and give the same 0.
	var ip interpolation
	ip.addPoint(10, 0.5, clipper.IntPoint{}, false, false)
	ip.addPoint(-10, -0.5, clipper.IntPoint{}, false, false)
	if ip.min.errorVal != -10 || ip.max.errorVal != 10 {
		t.Fatalf("samples not ordered (min,max): min=%g max=%g", ip.min.errorVal, ip.max.errorVal)
	}
	if got := ip.interpolateAngle(); math.Abs(got) > 1e-9 {
		t.Fatalf("interpolated angle = %g, want 0", got)
	}
}

func TestInterpolationClampAngle(t *testing.T) {
	var ip interpolation
	if ip.clampAngle(1.0) != interpMaxAngle {
		t.Fatal("angle above +45° should clamp to max")
	}
	if ip.clampAngle(-1.0) != interpMinAngle {
		t.Fatal("angle below -45° should clamp to min")
	}
	if ip.clampAngle(0.1) != 0.1 {
		t.Fatal("an in-range angle should be unchanged")
	}
}

func TestInterpolationClearAndInterpFractionClamp(t *testing.T) {
	var ip interpolation
	// Strongly asymmetric errors push the secant fraction p ≈ 1/1001 below the 0.2 clamp, so it
	// is clamped to 0.2: angle = -0.4·0.8 + 0.4·0.2 = -0.24 (not the ≈ -0.399 a pure secant gives).
	ip.addPoint(-1, -0.4, clipper.IntPoint{}, false, false)
	ip.addPoint(1000, 0.4, clipper.IntPoint{}, false, false)
	if got := ip.interpolateAngle(); math.Abs(got-(-0.24)) > 1e-9 {
		t.Fatalf("interpolated angle = %g, want -0.24 (secant fraction clamped to %.2f)", got, interpMinInterp)
	}
	ip.clear()
	if ip.getPointCount() != 0 {
		t.Fatal("clear should drop all samples")
	}
}

func TestDistancePointToPathsSqrd(t *testing.T) {
	// A 100x100 square; a point just outside the right edge is closest to that edge.
	sq := clipper.Paths{{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 100}, {X: 0, Y: 100}}}
	got := distancePointToPathsSqrd(sq, clipper.IntPoint{X: 110, Y: 50})
	if math.Abs(got.distSqrd-100) > 1e-6 {
		t.Fatalf("distance² = %g, want 100 (10 units from the right edge)", got.distSqrd)
	}
	if got.point != (clipper.IntPoint{X: 100, Y: 50}) {
		t.Fatalf("closest point = %v, want (100,50)", got.point)
	}
}

func TestDistancePointToPathsSqrdPicksNearestEdge(t *testing.T) {
	sq := clipper.Paths{{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 100}, {X: 0, Y: 100}}}
	// A point near the bottom edge inside the square is closest to that edge (distance 5).
	got := distancePointToPathsSqrd(sq, clipper.IntPoint{X: 50, Y: 5})
	if math.Abs(got.distSqrd-25) > 1e-6 {
		t.Fatalf("distance² = %g, want 25", got.distSqrd)
	}
}
