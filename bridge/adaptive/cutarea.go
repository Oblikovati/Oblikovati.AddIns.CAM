// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import (
	"math"
	"sort"

	"oblikovati.org/cam/bridge/clipper"
)

// calcCutArea computes the material area freshly cut by moving the tool (radius toolRadiusScaled,
// scaled units) from c1 to c2: the area inside the tool circle at c2 but outside the tool circle
// at c1 AND outside the already-cleared polygons. It also returns the portion of that area on the
// conventional-milling side of the move, which the solver uses to bias toward climb milling.
//
// boundedCleared is the cleared-region geometry already restricted to the tool neighbourhood (the
// cleared-area model supplies it); calcCutArea is otherwise a pure analytic integration and does
// not touch the clipping engine. This is the heart of engagement control — it is evaluated for
// every candidate tool move — and is an exact port of Adaptive2d::CalcCutArea: rotate the c1→c2
// vector to +Y, find every x of interest, and over each x-slab sweep the y-crossings of the two
// circles and the polygons, accumulating signed trapezoid/segment areas where the slab lies
// inside c2 yet outside everything else.
func calcCutArea(c1, c2 clipper.IntPoint, toolRadiusScaled int64, boundedCleared clipper.Paths) (area, conventionalArea float64) {
	if distanceBetween(c1, c2) < numericTolerance {
		return 0, 0
	}
	tr := float64(toolRadiusScaled)

	// 0) Keep only cleared polygons whose bounding box can reach the tool circle at c2.
	c2BB := newBoundBoxCircle(c2, toolRadiusScaled)
	var polygons [][]DoublePoint
	for _, path := range boundedCleared {
		if len(path) == 0 {
			continue
		}
		pbb := newBoundBoxPoint(path[0])
		for _, pt := range path {
			pbb.addPoint(pt)
		}
		if !pbb.collidesWith(c2BB) {
			continue
		}
		poly := make([]DoublePoint, len(path))
		for i, p := range path {
			poly[i] = DoublePoint{X: float64(p.X), Y: float64(p.Y)}
		}
		polygons = append(polygons, poly)
	}

	// 0.5) Rotate all geometry so the vector c1→c2 points up (+Y).
	angle := math.Pi/2 - math.Atan2(float64(c2.Y-c1.Y), float64(c2.X-c1.X))
	ca, sa := math.Cos(angle), math.Sin(angle)
	c1 = clipper.IntPoint{X: int64(ca*float64(c1.X) - sa*float64(c1.Y)), Y: int64(sa*float64(c1.X) + ca*float64(c1.Y))}
	c2 = clipper.IntPoint{X: int64(ca*float64(c2.X) - sa*float64(c2.Y)), Y: int64(sa*float64(c2.X) + ca*float64(c2.Y))}
	for _, poly := range polygons {
		for i, p := range poly {
			poly[i] = DoublePoint{X: ca*p.X - sa*p.Y, Y: sa*p.X + ca*p.Y}
		}
	}

	// 1) Collect every x-coordinate where the slab topology can change.
	var xs []float64
	for _, poly := range polygons {
		for _, p := range poly {
			xs = append(xs, p.X) // 1.a polygon vertices
		}
		for i := range poly { // 1.b/1.c polygon edges crossing each tool circle
			p0, p1 := poly[i], poly[(i+1)%len(poly)]
			for _, ip := range line2CircleIntersect(c1, tr, p0, p1, true) {
				xs = append(xs, ip.X)
			}
			for _, ip := range line2CircleIntersect(c2, tr, p0, p1, true) {
				xs = append(xs, ip.X)
			}
		}
	}
	if first, second, ok := circle2CircleIntersect(c1, c2, tr); ok { // 1.e the two circles
		xs = append(xs, first.X, second.X)
	}
	xs = append(xs, float64(c1.X)-tr, float64(c1.X)+tr) // 1.f c1 tangents
	xmin := float64(c2.X) - tr
	xmax := float64(c2.X) + tr
	xs = append(xs, xmin, xmax, float64(c2.X)) // c2 tangents and centre

	// 2) Keep only x in [xmin, xmax] (the c2 circle's span) and sort.
	filtered := xs[:0:0]
	for _, x := range xs {
		if xmin <= x && x <= xmax {
			filtered = append(filtered, x)
		}
	}
	xs = filtered
	sort.Float64s(xs)

	circles := []DoublePoint{{X: float64(c2.X), Y: float64(c2.Y)}, {X: float64(c1.X), Y: float64(c1.Y)}}
	// 3) Integrate over each x-slab through its midpoint.
	for ix := 0; ix+1 < len(xs); ix++ {
		x0, x1 := xs[ix], xs[ix+1]
		if x0 == x1 {
			continue
		}
		dA, dConv := integrateSlab(x0, x1, tr, polygons, circles)
		area += dA
		conventionalArea += dConv
	}
	return area, conventionalArea
}

// crossing is one y-intersection of the slab's midpoint vertical line with a shape: ishape indexes
// polygons first then circles (len(polygons)+icircle), ipart is the polygon edge index or, for a
// circle, 0 for the upper half and 1 for the lower.
type crossing struct {
	y             float64
	ishape, ipart int
}

// integrateSlab accumulates the signed cut area (and its conventional-side part) contributed by
// the vertical x-slab [x0,x1], by sweeping the y-crossings at the slab midpoint and adding area
// only where the running cover state is inside the new tool circle (c2) and outside everything
// else. Ported from the inner loop of CalcCutArea.
func integrateSlab(x0, x1, tr float64, polygons [][]DoublePoint, circles []DoublePoint) (area, conventionalArea float64) {
	xtest := (x0 + x1) / 2
	var ys []crossing
	for ip, poly := range polygons {
		for ie := range poly {
			p0, p1 := poly[ie], poly[(ie+1)%len(poly)]
			if math.Min(p0.X, p1.X) < xtest && math.Max(p0.X, p1.X) > xtest {
				ys = append(ys, crossing{y: interpX(p0, p1, xtest), ishape: ip, ipart: ie})
			}
		}
	}
	for ic, c := range circles {
		if dx := math.Abs(xtest - c.X); dx < tr {
			dy := math.Sqrt(tr*tr - dx*dx)
			ys = append(ys, crossing{y: c.Y + dy, ishape: len(polygons) + ic, ipart: 0})
			ys = append(ys, crossing{y: c.Y - dy, ishape: len(polygons) + ic, ipart: 1})
		}
	}
	sort.Slice(ys, func(i, j int) bool { return ys[i].y < ys[j].y })

	// Init: outside c2, inside every other shape; outsideCount counts shapes we are currently
	// outside of (c2 is index len(polygons)).
	outside := make([]bool, len(polygons)+len(circles))
	outside[len(polygons)] = true
	outsideCount := 1
	c2X := circles[0].X

	for _, cr := range ys {
		prevOutside := outside[cr.ishape]
		prevCount := outsideCount
		outside[cr.ishape] = !outside[cr.ishape]
		if prevOutside {
			outsideCount--
		} else {
			outsideCount++
		}
		if outsideCount != 0 && prevCount != 0 {
			continue
		}
		sign := 1.0 // entrance/exit sign: -integral(entrance) + integral(exit)
		if prevOutside {
			sign = -1.0
		}
		var newArea float64
		if cr.ishape < len(polygons) {
			poly := polygons[cr.ishape]
			p0, p1 := poly[cr.ipart], poly[(cr.ipart+1)%len(poly)]
			y0, y1 := interpX(p0, p1, x0), interpX(p0, p1, x1)
			newArea = (y0 + y1) / 2 * (x1 - x0)
		} else {
			newArea = circleSlabArea(circles[cr.ishape-len(polygons)], cr.ipart, x0, x1, tr)
		}
		area += sign * newArea
		if xtest < c2X {
			conventionalArea += sign * newArea
		}
	}
	return area, conventionalArea
}

// interpX is the y of the line through p0,p1 at abscissa x (linear interpolation).
func interpX(p0, p1 DoublePoint, x float64) float64 {
	t := (x - p0.X) / (p1.X - p0.X)
	return p1.Y*t + p0.Y*(1-t)
}

// circleSlabArea is the signed area under a circular arc over [x0,x1]: the circular segment (with
// upper/lower sign from half) plus the trapezoid down to y=0. half is 0 for the upper arc, 1 for
// the lower. Ported from the circle branch of CalcCutArea.
func circleSlabArea(c DoublePoint, half int, x0, x1, tr float64) float64 {
	circleSign := 1.0
	if half != 0 {
		circleSign = -1.0
	}
	clamp := func(a float64) float64 { return math.Max(-1.0, math.Min(1.0, a)) }
	phi0 := math.Acos(clamp((x0-c.X)/tr)) * circleSign
	phi1 := math.Acos(clamp((x1-c.X)/tr)) * circleSign
	areaSector := tr * tr / 2 * math.Abs(phi1-phi0)
	y0 := c.Y + circleSign*math.Sqrt(tr*tr-(x0-c.X)*(x0-c.X))
	y1 := c.Y + circleSign*math.Sqrt(tr*tr-(x1-c.X)*(x1-c.X))
	tbase := math.Sqrt((x1-x0)*(x1-x0) + (y1-y0)*(y1-y0))
	tmidx := (x0 + x1) / 2
	tmidy := (y0 + y1) / 2
	th := math.Sqrt((tmidx-c.X)*(tmidx-c.X) + (tmidy-c.Y)*(tmidy-c.Y))
	areaTriangle := tbase * th / 2
	areaSegment := areaSector - areaTriangle
	areaTrapezoid := (x1 - x0) * (y0 + y1) / 2
	return circleSign*areaSegment + areaTrapezoid
}
