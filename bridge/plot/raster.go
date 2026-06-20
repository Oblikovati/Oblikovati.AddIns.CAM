// SPDX-License-Identifier: GPL-2.0-only

package plot

import (
	"image"
	"image/color"
	"math"
)

// pt is an integer pixel coordinate.
type pt struct{ x, y int }

// transform maps world XY (millimetres) to pixels, flipping Y so +Y points up, and centring the
// content with a margin.
type transform struct {
	minX, minY float64
	scale      float64
	size       int
	margin     int
}

// box is an axis-aligned world-space bounding box.
type box struct{ minX, minY, maxX, maxY float64 }

// newTransform fits a world box into a size×size image with a fixed pixel margin, preserving
// aspect (uniform scale).
func newTransform(b box, size int) transform {
	const margin = 24
	w, h := b.maxX-b.minX, b.maxY-b.minY
	span := math.Max(math.Max(w, h), 1e-6)
	scale := float64(size-2*margin) / span
	// centre the smaller axis
	return transform{minX: b.minX - (span-w)/2, minY: b.minY - (span-h)/2, scale: scale, size: size, margin: margin}
}

// px maps a world point to a pixel, flipping Y.
func (t transform) px(x, y float64) pt {
	ix := t.margin + int((x-t.minX)*t.scale)
	iy := t.size - t.margin - int((y-t.minY)*t.scale)
	return pt{ix, iy}
}

// bounds is the world box enclosing the scene's boundary and every addressed XY in the path,
// padded slightly so strokes near the edge stay inside the image.
func bounds(s Scene) box {
	b := box{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	for _, p := range s.Boundary {
		b.add(p.X, p.Y)
	}
	var x, y float64
	for _, c := range s.Path.Commands {
		nx, ny, _, hasXY := next(c, x, y, 0)
		if hasXY {
			b.add(nx, ny)
		}
		x, y = nx, ny
	}
	if math.IsInf(b.minX, 1) {
		b = box{0, 0, 1, 1}
	}
	return b.pad(2)
}

// add expands the box to include (x,y).
func (b *box) add(x, y float64) {
	b.minX, b.minY = math.Min(b.minX, x), math.Min(b.minY, y)
	b.maxX, b.maxY = math.Max(b.maxX, x), math.Max(b.maxY, y)
}

// pad grows the box by m millimetres on every side.
func (b box) pad(m float64) box {
	return box{b.minX - m, b.minY - m, b.maxX + m, b.maxY + m}
}

// fill paints the whole image one colour.
func fill(img *image.RGBA, c color.RGBA) {
	r := img.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

// line draws a 1px Bresenham segment between two pixels.
func line(img *image.RGBA, a, b pt, c color.RGBA) {
	dx, dy := abs(b.x-a.x), -abs(b.y-a.y)
	sx, sy := step(a.x, b.x), step(a.y, b.y)
	err := dx + dy
	for {
		set(img, a.x, a.y, c)
		if a.x == b.x && a.y == b.y {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			a.x += sx
		}
		if e2 <= dx {
			err += dx
			a.y += sy
		}
	}
}

// dot draws a small filled square marker centred on a pixel.
func dot(img *image.RGBA, p pt, c color.RGBA) {
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			set(img, p.x+dx, p.y+dy, c)
		}
	}
}

// set writes one pixel, clipping to the image bounds.
func set(img *image.RGBA, x, y int, c color.RGBA) {
	if (image.Point{x, y}).In(img.Bounds()) {
		img.SetRGBA(x, y, c)
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func step(a, b int) int {
	if a < b {
		return 1
	}
	return -1
}
