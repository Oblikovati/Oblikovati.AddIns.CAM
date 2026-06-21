// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

package clipper

import (
	"math"
	"testing"
)

// rect builds a CCW axis-aligned rectangle from (x0,y0) to (x1,y1).
func rect(x0, y0, x1, y1 int64) Path {
	return Path{{x0, y0, 0}, {x1, y0, 0}, {x1, y1, 0}, {x0, y1, 0}}
}

// sumArea totals the signed areas of a path set (holes count negative).
func sumArea(ps Paths) float64 {
	a := 0.0
	for _, p := range ps {
		a += Area(p)
	}
	return a
}

func TestBooleanUnion(t *testing.T) {
	// Two 100x100 squares overlapping in a 50x50 corner: union area = 10000+10000-2500.
	got, err := Unite(Paths{rect(0, 0, 100, 100)}, Paths{rect(50, 50, 150, 150)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("union produced %d paths, want 1 (single L-shape)", len(got))
	}
	if a := math.Abs(Area(got[0])); a != 17500 {
		t.Fatalf("union area = %g, want 17500", a)
	}
}

// TestZTagSurvivesUnion proves the use_xyz tag the adaptive solver threads survives a boolean and
// the cgo round trip: a retained subject vertex keeps its Z, while the clip vertices and any new
// intersection points stay at 0 (no fill callback) — exactly what Execute steps 4–5 rely on to
// distinguish profile walls (Z=1, needs finishing) from stock boundaries (Z=0).
func TestZTagSurvivesUnion(t *testing.T) {
	subject := rect(0, 0, 100, 100)
	for i := range subject {
		subject[i].Z = 1 // tag the whole profile wall as needing a finishing pass
	}
	clip := rect(50, 50, 150, 150) // a stock boundary at the default Z=0

	got, err := Unite(Paths{subject}, Paths{clip})
	if err != nil {
		t.Fatal(err)
	}
	sawOne, sawZero := false, false
	for _, p := range got {
		for _, pt := range p {
			switch pt.Z {
			case 1:
				sawOne = true
			case 0:
				sawZero = true
			}
		}
	}
	if !sawOne {
		t.Error("a retained subject vertex should keep its Z=1 tag through the union")
	}
	if !sawZero {
		t.Error("clip vertices and new intersection points should carry Z=0")
	}
}

func TestBooleanDifferenceLeavesHole(t *testing.T) {
	// 100x100 outer minus a centred 50x50 inner: an outer ring plus a hole of opposite winding.
	got, err := Subtract(Paths{rect(0, 0, 100, 100)}, Paths{rect(25, 25, 75, 75)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("difference produced %d paths, want 2 (outer + hole)", len(got))
	}
	if net := sumArea(got); net != 7500 {
		t.Fatalf("net area = %g, want 7500 (10000 outer - 2500 hole)", net)
	}
}

func TestBooleanIntersection(t *testing.T) {
	got, err := Intersect(Paths{rect(0, 0, 100, 100)}, Paths{rect(50, 50, 150, 150)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || math.Abs(Area(got[0])) != 2500 {
		t.Fatalf("intersection = %v, want one 50x50 (area 2500) path", got)
	}
}

func TestBooleanOpenPathClippedToRegion(t *testing.T) {
	// A horizontal open polyline crossing a 100x100 region: the inside segment is x in [0,100].
	open := Paths{{{-50, 50, 0}, {150, 50, 0}}}
	got, err := Boolean(Intersection, EvenOdd, open, false, Paths{rect(0, 0, 100, 100)}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0]) != 2 {
		t.Fatalf("open clip = %v, want one 2-point segment", got)
	}
	lo, hi := got[0][0].X, got[0][1].X
	if lo > hi {
		lo, hi = hi, lo
	}
	if lo != 0 || hi != 100 {
		t.Fatalf("clipped segment x-span = [%d,%d], want [0,100]", lo, hi)
	}
}

func TestOffsetShrinksSquare(t *testing.T) {
	// Inset a 100x100 square by 10: ~80x80, area ~6400. Round join leaves the inward corners
	// sharp, so the result is close to exact.
	got, err := OffsetClosed(Paths{rect(0, 0, 100, 100)}, -10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("offset produced %d paths, want 1", len(got))
	}
	if a := math.Abs(Area(got[0])); a < 6200 || a > 6500 {
		t.Fatalf("inset square area = %g, want ~6400", a)
	}
}

func TestOffsetTooSmallVanishes(t *testing.T) {
	// Shrinking a 100x100 square by 60 removes it entirely (no room left).
	got, err := OffsetClosed(Paths{rect(0, 0, 100, 100)}, -60)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("over-shrink produced %d paths, want 0", len(got))
	}
}

func TestSimplifyResolvesSelfIntersection(t *testing.T) {
	// A bowtie (self-intersecting quad) resolves into two triangles under even-odd fill.
	bowtie := Paths{{{0, 0, 0}, {100, 0, 0}, {0, 100, 0}, {100, 100, 0}}}
	got, err := Simplify(bowtie, EvenOdd)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("simplify of a bowtie produced %d paths, want 2 triangles", len(got))
	}
}

func TestEmptyInputsAreSafe(t *testing.T) {
	got, err := Unite(Paths{}, Paths{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("union of nothing = %v, want empty", got)
	}
}

func TestPathIntersectAreaWholeRingInside(t *testing.T) {
	// An engage-style closed ring fully inside the region comes back as one open path (rejoined
	// across the closing seam) that visits all the ring's vertices in order.
	ring := Path{{20, 20, 0}, {80, 20, 0}, {80, 80, 0}, {20, 80, 0}}
	got, err := PathIntersectArea(ring, Paths{rect(0, 0, 100, 100)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("a wholly-inside ring = %d paths, want 1", len(got))
	}
	if len(got[0]) < 4 {
		t.Fatalf("rejoined ring too short: %v", got[0])
	}
}

func TestPathIntersectAreaClipsRingToRegion(t *testing.T) {
	// A ring straddling the region edge keeps only its inside portion; every returned point lies
	// within the region (x,y in [0,100]).
	ring := Path{{50, 50, 0}, {200, 50, 0}, {200, 200, 0}, {50, 200, 0}}
	got, err := PathIntersectArea(ring, Paths{rect(0, 0, 100, 100)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("a ring straddling the region should leave an inside portion")
	}
	for _, p := range got {
		for _, pt := range p {
			if pt.X < 0 || pt.X > 100 || pt.Y < 0 || pt.Y > 100 {
				t.Fatalf("clipped point %v fell outside the region", pt)
			}
		}
	}
}

func TestPathIntersectAreaOutsideIsEmpty(t *testing.T) {
	ring := Path{{200, 200, 0}, {300, 200, 0}, {300, 300, 0}, {200, 300, 0}}
	got, err := PathIntersectArea(ring, Paths{rect(0, 0, 100, 100)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("a ring entirely outside the area = %v, want empty", got)
	}
}
