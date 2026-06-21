// SPDX-License-Identifier: GPL-2.0-only

// Package geom2d is the pure 2D-geometry core of the CAM add-in: a Polygon (closed loop of
// points) with the operations 2.5D toolpaths need — signed area, winding, point
// containment, and polygon offsetting. It is the leaf the profile/pocket generators build
// on, depends on nothing in this module, and is fully unit-testable without a host. It
// provides 2D offset + region math, in pure Go.
package geom2d

import "math"

// Point2 is a 2D point in millimetres (the toolpath/G-code convention).
type Point2 struct {
	X, Y float64
}

// Polygon is a closed loop of points; the closing edge from the last point back to the
// first is implicit (do not repeat the first point at the end).
type Polygon []Point2

// epsilon is the tolerance for area/winding/parallel degeneracy checks (mm-scale).
const epsilon = 1e-9

// SignedArea returns the polygon's signed area via the shoelace formula: positive when the
// vertices wind counter-clockwise (CCW), negative for clockwise. Magnitude is the area.
func (p Polygon) SignedArea() float64 {
	if len(p) < 3 {
		return 0
	}
	var sum float64
	for i := range p {
		a := p[i]
		b := p[(i+1)%len(p)]
		sum += a.X*b.Y - b.X*a.Y
	}
	return sum / 2
}

// Area is the unsigned area.
func (p Polygon) Area() float64 { return math.Abs(p.SignedArea()) }

// IsCCW reports whether the polygon winds counter-clockwise.
func (p Polygon) IsCCW() bool { return p.SignedArea() > 0 }

// Reversed returns a copy wound in the opposite direction.
func (p Polygon) Reversed() Polygon {
	out := make(Polygon, len(p))
	for i := range p {
		out[len(p)-1-i] = p[i]
	}
	return out
}

// EnsureCCW returns the polygon wound counter-clockwise (reversing a CW one). Offsetting is
// defined relative to a known winding, so callers normalise first.
func (p Polygon) EnsureCCW() Polygon {
	if p.IsCCW() {
		return p
	}
	return p.Reversed()
}

// Perimeter is the total edge length including the closing edge.
func (p Polygon) Perimeter() float64 {
	if len(p) < 2 {
		return 0
	}
	var sum float64
	for i := range p {
		sum += dist(p[i], p[(i+1)%len(p)])
	}
	return sum
}

// Centroid returns the area centroid of the polygon (falls back to the vertex average for a
// degenerate, near-zero-area loop).
func (p Polygon) Centroid() Point2 {
	a := p.SignedArea()
	if math.Abs(a) < epsilon {
		return p.vertexAverage()
	}
	var cx, cy float64
	for i := range p {
		j := (i + 1) % len(p)
		cross := p[i].X*p[j].Y - p[j].X*p[i].Y
		cx += (p[i].X + p[j].X) * cross
		cy += (p[i].Y + p[j].Y) * cross
	}
	return Point2{X: cx / (6 * a), Y: cy / (6 * a)}
}

// vertexAverage is the mean of the vertices.
func (p Polygon) vertexAverage() Point2 {
	if len(p) == 0 {
		return Point2{}
	}
	var sx, sy float64
	for _, v := range p {
		sx += v.X
		sy += v.Y
	}
	return Point2{X: sx / float64(len(p)), Y: sy / float64(len(p))}
}

// Contains reports whether the point lies strictly inside the polygon (ray-casting).
// Boundary points are not guaranteed either way.
func (p Polygon) Contains(pt Point2) bool {
	if len(p) < 3 {
		return false
	}
	inside := false
	for i, j := 0, len(p)-1; i < len(p); j, i = i, i+1 {
		yi, yj := p[i].Y, p[j].Y
		if (yi > pt.Y) != (yj > pt.Y) {
			xCross := (p[j].X-p[i].X)*(pt.Y-yi)/(yj-yi) + p[i].X
			if pt.X < xCross {
				inside = !inside
			}
		}
	}
	return inside
}

// dist is the Euclidean distance between two points.
func dist(a, b Point2) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }
