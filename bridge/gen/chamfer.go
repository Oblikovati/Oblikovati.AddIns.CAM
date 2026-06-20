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
}

// defaultChamferAngleDeg is the V-tool included angle used when ToolAngleDeg is unset — 90°,
// whose 45° half-angle makes the chamfer depth equal its width.
const defaultChamferAngleDeg = 90.0

// GenerateChamfer cuts a single bevel pass around the boundary: the tip path is the boundary
// offset to the edge side by the chamfer width, walked once at the chamfer depth
// (width / tan(halfAngle)) below the top. This is the add-in analogue of FreeCAD's chamfer /
// deburr operation (the single-flank bevel).
func GenerateChamfer(boundary geom2d.Polygon, top float64, feeds Feeds, p ChamferParams) ([]gcode.Command, error) {
	if p.Width <= 0 {
		return nil, fmt.Errorf("chamfer needs a positive width, got %g", p.Width)
	}
	if len(boundary) < 3 {
		return nil, fmt.Errorf("chamfer boundary needs at least 3 points, got %d", len(boundary))
	}
	half := chamferHalfAngle(p.ToolAngleDeg)
	z := top - p.Width/math.Tan(half)
	tip, err := chamferTipPath(boundary, p)
	if err != nil {
		return nil, err
	}
	return walkLoop(orient(tip, p.Climb), z, feeds), nil
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
