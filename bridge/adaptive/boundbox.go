// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import "oblikovati.org/cam/bridge/clipper"

// boundBox is an axis-aligned bounding box in the scaled integer plane, used for the cheap
// collision/containment pre-checks that keep the engagement integration and the cleared-area
// cache from touching geometry that cannot matter. Exact port of the solver's BoundBox.
type boundBox struct {
	minX, maxX, minY, maxY int64
}

// newBoundBoxPoint starts a box at a single point (grow it with addPoint).
func newBoundBoxPoint(p clipper.IntPoint) boundBox {
	return boundBox{minX: p.X, maxX: p.X, minY: p.Y, maxY: p.Y}
}

// newBoundBoxCircle is the box enclosing a circle of the given radius about center.
func newBoundBoxCircle(center clipper.IntPoint, radius int64) boundBox {
	return boundBox{minX: center.X - radius, maxX: center.X + radius, minY: center.Y - radius, maxY: center.Y + radius}
}

// setFirstPoint resets the box to a single point.
func (b *boundBox) setFirstPoint(p clipper.IntPoint) {
	b.minX, b.maxX, b.minY, b.maxY = p.X, p.X, p.Y, p.Y
}

// addPoint grows the box to include pt.
func (b *boundBox) addPoint(pt clipper.IntPoint) {
	b.minX = min64(pt.X, b.minX)
	b.maxX = max64(pt.X, b.maxX)
	b.minY = min64(pt.Y, b.minY)
	b.maxY = max64(pt.Y, b.maxY)
}

// collidesWith reports whether the two boxes overlap (touching edges count).
func (b boundBox) collidesWith(o boundBox) bool {
	return b.minX <= o.maxX && b.maxX >= o.minX && b.minY <= o.maxY && b.maxY >= o.minY
}

// contains reports whether b fully encloses o.
func (b boundBox) contains(o boundBox) bool {
	return b.minX <= o.minX && b.maxX >= o.maxX && b.minY <= o.minY && b.maxY >= o.maxY
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
