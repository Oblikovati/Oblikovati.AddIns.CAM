// SPDX-License-Identifier: GPL-2.0-only

// Package plot renders a toolpath (gcode.Path) to a raster image for visual validation —
// rapids, cutting moves, arcs, and plunge points in distinct colours over the driving boundary.
// It is pure Go (image/png only) so it runs anywhere, turning a generated program into a
// picture a human (or Claude) can eyeball against the intended shape.
package plot

import (
	"image"
	"image/color"
	"math"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/geom2d"
)

// Scene is one labelled toolpath to render: the program plus the boundary it was generated for.
type Scene struct {
	Path     gcode.Path
	Boundary geom2d.Polygon
}

// palette holds the colours of the rendered toolpath elements.
var (
	colBackground = color.RGBA{0x12, 0x14, 0x18, 0xff}
	colBoundary   = color.RGBA{0x55, 0x59, 0x60, 0xff}
	colRapid      = color.RGBA{0xd6, 0x4b, 0x4b, 0xff} // G0 — red
	colCut        = color.RGBA{0x3d, 0x8b, 0xff, 0xff} // G1/G2/G3 XY at full depth — blue
	colElevated   = color.RGBA{0xf2, 0x9d, 0x38, 0xff} // cut above the floor (tab lift / ramp) — orange
	colPlunge     = color.RGBA{0x35, 0xc4, 0x6a, 0xff} // plunge point — green
)

// Render rasterises a scene into a size×size RGBA image, fitting the boundary and toolpath with
// a margin. The boundary is drawn first (faint), then the moves over it.
func Render(s Scene, size int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	fill(img, colBackground)
	tf := newTransform(bounds(s), size)
	drawBoundary(img, s.Boundary, tf)
	drawMoves(img, s.Path, tf)
	return img
}

// drawBoundary outlines the driving region as a faint closed loop.
func drawBoundary(img *image.RGBA, poly geom2d.Polygon, tf transform) {
	if len(poly) < 2 {
		return
	}
	for i := range poly {
		a, b := poly[i], poly[(i+1)%len(poly)]
		line(img, tf.px(a.X, a.Y), tf.px(b.X, b.Y), colBoundary)
	}
}

// drawMoves walks the program, drawing each move from the running position in its move-type
// colour and marking plunge points. Cutting moves are shaded by depth (lighter = higher Z) so
// Z-only dressups — holding-tab lifts, ramped descents — are visible in the flat top-down view.
func drawMoves(img *image.RGBA, path gcode.Path, tf transform) {
	zr := cutZRange(path)
	var elevated []segment // elevated (above-floor) cut moves, redrawn last so they're never hidden
	var x, y, z float64
	known := false
	for _, c := range path.Commands {
		nx, ny, nz, hasXY := next(c, x, y, z)
		switch {
		case isDrillCycle(c):
			dot(img, tf.px(nx, ny), colPlunge) // a canned drill cycle bores at the hole
		case hasXY && known && c.Name == "G0":
			line(img, tf.px(x, y), tf.px(nx, ny), colRapid)
		case hasXY && known && isArc(c):
			drawArc(img, x, y, nx, ny, c, tf)
		case hasXY && known:
			col := shadeCut(nz, zr)
			line(img, tf.px(x, y), tf.px(nx, ny), col)
			if col == colElevated {
				elevated = append(elevated, segment{tf.px(x, y), tf.px(nx, ny)})
			}
		case !hasXY && nz < z && known:
			dot(img, tf.px(x, y), colPlunge) // a plunge: Z drops with no XY
		}
		x, y, z = nx, ny, nz
		known = known || hasXY
	}
	for _, s := range elevated { // overlay so a ramp/tab lift shows over a later full-depth pass
		line(img, s.a, s.b, colElevated)
	}
}

// segment is a pixel-space line, retained for the elevated-cut overlay pass.
type segment struct{ a, b pt }

// zSpan is the min/max Z of a path's cutting moves.
type zSpan struct{ lo, hi float64 }

// cutZRange returns the Z range over the cutting (G1/G2/G3 XY) moves, used to shade by depth.
func cutZRange(path gcode.Path) zSpan {
	zr := zSpan{math.Inf(1), math.Inf(-1)}
	var x, y, z float64
	for _, c := range path.Commands {
		nx, ny, nz, hasXY := next(c, x, y, z)
		if hasXY && (c.Name == "G1" || isArc(c)) {
			zr.lo, zr.hi = math.Min(zr.lo, nz), math.Max(zr.hi, nz)
		}
		x, y, z = nx, ny, nz
	}
	return zr
}

// shadeCut colours a cut move by depth: blue at the floor (the deepest pass), orange when it
// runs above the floor. This keeps the whole path visible while flagging the Z-only dressups —
// a holding-tab lift or a ramped entry shows as orange segments on the blue loop. A flat
// (single-Z) path is all blue.
func shadeCut(z float64, zr zSpan) color.RGBA {
	if math.IsInf(zr.lo, 1) || z <= zr.lo+1e-6 {
		return colCut
	}
	return colElevated
}

// isDrillCycle reports whether a command is a canned drilling cycle (G73, G81–G89), which bores
// at its X/Y rather than tracing an XY path.
func isDrillCycle(c gcode.Command) bool {
	switch c.Name {
	case "G73", "G81", "G82", "G83", "G84", "G85", "G86", "G87", "G88", "G89":
		return true
	}
	return false
}

// drawArc samples a G2/G3 arc (centre at begin+I,J) into short segments so leads and spirals
// render as curves rather than chords.
func drawArc(img *image.RGBA, x, y, nx, ny float64, c gcode.Command, tf transform) {
	cx, cy := x+c.Params["I"], y+c.Params["J"]
	r := math.Hypot(x-cx, y-cy)
	a0, a1 := math.Atan2(y-cy, x-cx), math.Atan2(ny-cy, nx-cx)
	ccw := c.Name == "G3"
	sweep := arcSweep(a0, a1, ccw)
	steps := int(math.Max(2, math.Abs(sweep)/(math.Pi/24)))
	px, py := x, y
	for i := 1; i <= steps; i++ {
		a := a0 + sweep*float64(i)/float64(steps)
		qx, qy := cx+r*math.Cos(a), cy+r*math.Sin(a)
		line(img, tf.px(px, py), tf.px(qx, qy), colCut)
		px, py = qx, qy
	}
}

// arcSweep returns the signed angular sweep from a0 to a1 in the arc's turn direction.
func arcSweep(a0, a1 float64, ccw bool) float64 {
	d := a1 - a0
	if ccw {
		for d <= 0 {
			d += 2 * math.Pi
		}
	} else {
		for d >= 0 {
			d -= 2 * math.Pi
		}
	}
	return d
}

// isArc reports whether a command is a G2/G3 arc move.
func isArc(c gcode.Command) bool { return c.Name == "G2" || c.Name == "G3" }

// next applies a command's X/Y/Z to the running position, reporting whether it moved in XY.
func next(c gcode.Command, x, y, z float64) (nx, ny, nz float64, hasXY bool) {
	nx, ny, nz = x, y, z
	if v, ok := c.Params["X"]; ok {
		nx, hasXY = v, true
	}
	if v, ok := c.Params["Y"]; ok {
		ny, hasXY = v, true
	}
	if v, ok := c.Params["Z"]; ok {
		nz = v
	}
	return nx, ny, nz, hasXY
}
