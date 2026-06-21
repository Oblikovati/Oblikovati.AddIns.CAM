// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import (
	"testing"

	"oblikovati.org/cam/bridge/clipper"
)

func TestBoundBoxGrowAndCircle(t *testing.T) {
	b := newBoundBoxPoint(clipper.IntPoint{X: 5, Y: 5})
	b.addPoint(clipper.IntPoint{X: -3, Y: 10})
	b.addPoint(clipper.IntPoint{X: 8, Y: -2})
	if b.minX != -3 || b.maxX != 8 || b.minY != -2 || b.maxY != 10 {
		t.Fatalf("grown box = %+v, want (-3,-2)-(8,10)", b)
	}
	c := newBoundBoxCircle(clipper.IntPoint{X: 0, Y: 0}, 100)
	if c.minX != -100 || c.maxX != 100 || c.minY != -100 || c.maxY != 100 {
		t.Fatalf("circle box = %+v", c)
	}
}

func TestBoundBoxSetFirstPoint(t *testing.T) {
	b := newBoundBoxPoint(clipper.IntPoint{X: 5, Y: 5})
	b.addPoint(clipper.IntPoint{X: 50, Y: 50})
	b.setFirstPoint(clipper.IntPoint{X: 1, Y: 2})
	if b != (boundBox{minX: 1, maxX: 1, minY: 2, maxY: 2}) {
		t.Fatalf("setFirstPoint did not reset the box: %+v", b)
	}
}

func TestBoundBoxCollideAndContain(t *testing.T) {
	a := boundBox{minX: 0, maxX: 10, minY: 0, maxY: 10}
	overlap := boundBox{minX: 5, maxX: 15, minY: 5, maxY: 15}
	apart := boundBox{minX: 20, maxX: 30, minY: 0, maxY: 10}
	inner := boundBox{minX: 2, maxX: 8, minY: 2, maxY: 8}
	if !a.collidesWith(overlap) {
		t.Fatal("overlapping boxes should collide")
	}
	if a.collidesWith(apart) {
		t.Fatal("disjoint boxes should not collide")
	}
	if !a.contains(inner) {
		t.Fatal("a should contain inner")
	}
	if a.contains(overlap) {
		t.Fatal("a should not contain a box that sticks out")
	}
}
