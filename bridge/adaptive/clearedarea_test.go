// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/clipper"
)

// engineAvailable reports whether the cgo clipping engine is linked into this build. The
// clearedArea tests assert real geometry when it is and a graceful error when it is not, so they
// are meaningful in both the cgo (race) and non-cgo (coverage) CI jobs.
func engineAvailable() bool {
	_, err := clipper.Unite(clipper.Paths{{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}}}, nil)
	return err == nil
}

func totalArea(ps clipper.Paths) float64 {
	a := 0.0
	for _, p := range ps {
		a += math.Abs(clipper.Area(p))
	}
	return a
}

func TestClearedAreaSetAndGet(t *testing.T) {
	ca := newClearedArea(100)
	sq := clipper.Paths{{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 100}, {X: 0, Y: 100}}}
	ca.setClearedPaths(sq)
	if got := ca.cleared(); len(got) != 1 || math.Abs(clipper.Area(got[0])) != 10000 {
		t.Fatalf("cleared() = %v, want the 100x100 square", got)
	}
}

func TestClearedAreaAddUnion(t *testing.T) {
	ca := newClearedArea(100)
	ca.setClearedPaths(clipper.Paths{{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 100}, {X: 0, Y: 100}}})
	err := ca.addClearedPaths(clipper.Paths{{{X: 50, Y: 50}, {X: 150, Y: 50}, {X: 150, Y: 150}, {X: 50, Y: 150}}})
	if !engineAvailable() {
		if err == nil {
			t.Fatal("addClearedPaths should error without the cgo engine")
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	// Two 100x100 squares overlapping in a 50x50 corner union to area 17500.
	if a := totalArea(ca.cleared()); math.Abs(a-17500) > 1 {
		t.Fatalf("unioned cleared area = %g, want 17500", a)
	}
}

func TestClearedAreaExpandGrows(t *testing.T) {
	ca := newClearedArea(100)
	// A straight tool pass of length 200, swept by a radius-101 footprint, clears a stadium:
	// rectangle(200 x 202) + a full circle(r=101) ≈ 40400 + 32047 ≈ 72400 (a touch less from the
	// faceted round cap).
	err := ca.expandCleared(clipper.Path{{X: 0, Y: 0}, {X: 200, Y: 0}})
	if !engineAvailable() {
		if err == nil {
			t.Fatal("expandCleared should error without the cgo engine")
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if a := totalArea(ca.cleared()); a < 70000 || a > 74000 {
		t.Fatalf("swept-pass cleared area = %g, want ~72400 (stadium)", a)
	}
}

func TestClearedAreaBoundedClip(t *testing.T) {
	ca := newClearedArea(100)
	ca.setClearedPaths(clipper.Paths{{{X: -1000, Y: -1000}, {X: 1000, Y: -1000}, {X: 1000, Y: 1000}, {X: -1000, Y: 1000}}})
	got, err := ca.boundedClearedAreaClipped(clipper.IntPoint{X: 0, Y: 0}, 100)
	if !engineAvailable() {
		if err == nil {
			t.Fatal("boundedClearedAreaClipped should error without the cgo engine")
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	// The 200x200 window fully inside the cleared region clips to a 200x200 square (area 40000).
	if a := totalArea(got); math.Abs(a-40000) > 1 {
		t.Fatalf("clipped window area = %g, want 40000", a)
	}
}

func TestClearedAreaExpandEmptyIsNoop(t *testing.T) {
	ca := newClearedArea(100)
	if err := ca.expandCleared(nil); err != nil {
		t.Fatalf("expanding by an empty path should be a no-op, got %v", err)
	}
	if len(ca.cleared()) != 0 {
		t.Fatal("cleared geometry should still be empty")
	}
}
