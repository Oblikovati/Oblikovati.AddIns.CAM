// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"fmt"
	"math"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/geom2d"
)

// ChamferParams configure a chamfer / edge-break pass cut with a V-shaped chamfer tool. The
// tool flank bevels the top edge of the contour: the tip traces the boundary offset to the edge
// side by the chamfer width, riding at a depth set by the tool's included angle so the flank
// produces a bevel exactly Width wide. A sharp-tipped V-tool is assumed (no tip radius).
type ChamferParams struct {
	Width        float64 // mm — horizontal width of the bevel face
	ToolAngleDeg float64 // included angle of the V-tool (degrees); <=0 → 90°
	Side         string  // SideOutside | SideInside | SideOn — which side of the boundary the edge faces
	Climb        bool    // climb vs conventional milling
	Passes       int     // number of flank passes to reach full width (>1 roughs a wide bevel); 0/1 → single pass
}

// defaultChamferAngleDeg is the V-tool included angle used when ToolAngleDeg is unset — 90°,
// whose 45° half-angle makes the chamfer depth equal its width.
const defaultChamferAngleDeg = 90.0

// maxChamferPasses caps the flank passes so a huge count can't explode the path.
const maxChamferPasses = 100

// GenerateChamfer cuts the bevel around the boundary with a V-tool. A single pass traces the
// boundary offset to the edge side by the chamfer width at the chamfer depth (width /
// tan(halfAngle)) below the top. With Passes > 1 the bevel is roughed in flank passes that step
// the offset and depth together from the top edge down to the full width — for a wide chamfer or
// a deburr where one full-flank cut would overload the tool.
func GenerateChamfer(boundary geom2d.Polygon, top float64, feeds Feeds, p ChamferParams) ([]gcode.Command, error) {
	if p.Width <= 0 {
		return nil, fmt.Errorf("chamfer needs a positive width, got %g", p.Width)
	}
	if len(boundary) < 3 {
		return nil, fmt.Errorf("chamfer boundary needs at least 3 points, got %d", len(boundary))
	}
	if _, err := chamferTipPath(boundary, p); err != nil { // the full-width pass must fit
		return nil, err
	}
	tan := math.Tan(chamferHalfAngle(p.ToolAngleDeg))
	n := chamferPasses(p.Passes)

	var cmds []gcode.Command
	for j := 1; j <= n; j++ {
		pass := p
		pass.Width = p.Width * float64(j) / float64(n) // narrower offset than the validated full pass
		tip, err := chamferTipPath(boundary, pass)
		if err != nil {
			continue
		}
		cmds = append(cmds, walkLoopVaryingZ(orient(tip, p.Climb), chamferDepthAt(boundary, pass.Width, top, tan), feeds)...)
	}
	return cmds, nil
}

// chamferDepthAt builds the per-point depth function for one flank pass. The bevel is Width wide, so
// the tip normally rides at a uniform depth Width/tan below the top. But the depth is *capped* by a
// tip point's true distance to the nearest wall: at a concave corner an inside-chamfer tip is closer
// to a crowding edge than its width, and cutting the full depth there would drive the V-tool flank
// into that wall. The min() only ever reduces the depth — on the common convex/outside case the
// nearest wall is at least the offset width away, so the depth stays uniform (no over-deepening at
// the wider diagonal of a convex offset corner).
func chamferDepthAt(boundary geom2d.Polygon, width, top, tan float64) func(geom2d.Point2) float64 {
	return func(pt geom2d.Point2) float64 {
		return top - math.Min(width, geom2d.DistanceToBoundary(pt, boundary))/tan
	}
}

// chamferPasses clamps the flank-pass count to at least one and no more than the cap.
func chamferPasses(passes int) int {
	if passes < 1 {
		return 1
	}
	if passes > maxChamferPasses {
		return maxChamferPasses
	}
	return passes
}

// chamferHalfAngle returns the V-tool half-angle in radians, defaulting to 45° (a 90° tool).
func chamferHalfAngle(angleDeg float64) float64 {
	if angleDeg <= 0 {
		angleDeg = defaultChamferAngleDeg
	}
	return angleDeg / 2 * math.Pi / 180
}

// chamferTipPath offsets the boundary to the edge side by the chamfer width — outward for an
// outside edge, inward for an inside one, unchanged for "on" (tip on the boundary line).
func chamferTipPath(boundary geom2d.Polygon, p ChamferParams) (geom2d.Polygon, error) {
	if p.Side == SideOn {
		return boundary.EnsureCCW(), nil
	}
	d := p.Width
	if p.Side == SideInside {
		d = -d
	}
	tip, ok := geom2d.Offset(boundary, d)
	if !ok {
		return nil, fmt.Errorf("chamfer side %q: width %g collapses the contour — too wide for the feature", p.Side, p.Width)
	}
	return tip, nil
}
