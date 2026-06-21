// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"fmt"
	"math"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/geom2d"
)

// VCarveParams configure V-carving: a V-bit traces the region as nested inward contours, each
// cut deeper the further it lies from the boundary, so the groove forms a V cross-section that is
// shallow at the edges and deepest along the region's spine. This is the offset-based V-carve
// (depth ∝ distance from the edge), used for engraved lettering and decorative reliefs.
type VCarveParams struct {
	ToolAngleDeg float64 // included angle of the V-bit (degrees); <=0 → 90°
	ToolDiameter float64 // mm — sets the radial step between contours
	StepOver     float64 // fraction of tool diameter between contours (0..1); 0 → 0.5
}

// GenerateVCarve carves the region inside the boundary as nested inward offset contours, until the
// offsets collapse at the region's spine. Each point is cut at a depth set by its true distance to
// the nearest wall (distance / tan(halfAngle)) below the top — not the contour's nominal offset —
// so the V-bit never digs into a nearby wall where two edges crowd a contour point (a concave
// corner), which a uniform per-contour depth would over-cut. The boundary is cut at the surface.
func GenerateVCarve(boundary geom2d.Polygon, top float64, feeds Feeds, p VCarveParams) ([]gcode.Command, error) {
	if p.ToolDiameter <= 0 {
		return nil, fmt.Errorf("v-carve needs a positive tool diameter, got %g", p.ToolDiameter)
	}
	if len(boundary) < 3 {
		return nil, fmt.Errorf("v-carve boundary needs at least 3 points, got %d", len(boundary))
	}
	half := chamferHalfAngle(p.ToolAngleDeg) // reuses the chamfer half-angle (defaults to 45°)
	step := vcarveStep(p.StepOver) * p.ToolDiameter
	tan := math.Tan(half)
	depthAt := func(pt geom2d.Point2) float64 { return top - geom2d.DistanceToBoundary(pt, boundary)/tan }

	var cmds []gcode.Command
	for d := 0.0; ; d += step {
		ring, ok := vcarveRing(boundary, d)
		if !ok {
			break
		}
		cmds = append(cmds, walkLoopVaryingZ(orient(ring, true), depthAt, feeds)...)
		if step <= 0 {
			break
		}
	}
	return cmds, nil
}

// vcarveRing returns the boundary offset inward by d (the boundary itself at d=0), reporting
// ok=false once the offset collapses at the spine.
func vcarveRing(boundary geom2d.Polygon, d float64) (geom2d.Polygon, bool) {
	if d <= 1e-9 {
		return boundary.EnsureCCW(), true
	}
	return geom2d.Offset(boundary, -d)
}

// vcarveStep returns the contour-spacing fraction, defaulting to 0.5.
func vcarveStep(frac float64) float64 {
	if frac <= 0 {
		return 0.5
	}
	return frac
}
