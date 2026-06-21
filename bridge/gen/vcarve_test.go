// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/geom2d"
)

// plungeDepths collects the Z of each contour's plunge move (a G1 carrying Z but no X/Y) — one per
// nested contour, in order from the surface inward.
func plungeDepths(cmds []gcode.Command) []float64 {
	var depths []float64
	for _, c := range cmds {
		_, hasX := c.Params["X"]
		if z, ok := c.Params["Z"]; ok && c.Name == "G1" && !hasX {
			depths = append(depths, z)
		}
	}
	return depths
}

// TestVCarveDeepensInward checks the boundary contour is cut at the surface and inner contours
// progressively deeper, forming the V cross-section. On a convex square every point on a contour
// is the offset distance from the wall, so depth = offset / tan(halfAngle).
func TestVCarveDeepensInward(t *testing.T) {
	cmds, err := GenerateVCarve(square(40), 0, testFeeds, VCarveParams{ToolAngleDeg: 90, ToolDiameter: 4, StepOver: 0.5})
	if err != nil {
		t.Fatalf("GenerateVCarve: %v", err)
	}
	depths := plungeDepths(cmds)
	if len(depths) < 3 {
		t.Fatalf("expected several nested contours, got %d", len(depths))
	}
	if !approx(depths[0], 0) {
		t.Errorf("boundary contour depth = %g, want 0 (surface)", depths[0])
	}
	for i := 1; i < len(depths); i++ {
		if depths[i] >= depths[i-1] {
			t.Errorf("contour %d plunge depth %g is not deeper than %g", i, depths[i], depths[i-1])
		}
	}
	// a 90° V-bit on a convex square: the second contour (offset 2mm) plunges at z=-2.
	if !approx(depths[1], -2) {
		t.Errorf("second contour depth = %g, want -2 (2mm offset / tan45°)", depths[1])
	}
}

// TestVCarveDepthFollowsNearestWall checks the fix: at a concave corner, a contour point is closer
// to the near wall than its nominal offset, so it is cut shallower than offset/tan — never deep
// enough for the V-bit flank to gouge that wall.
func TestVCarveDepthFollowsNearestWall(t *testing.T) {
	// An L-shaped region (a 40×40 square with a 20×20 bite out of the top-right) has one concave
	// corner at (20,20).
	l := geom2d.Polygon{{X: 0, Y: 0}, {X: 40, Y: 0}, {X: 40, Y: 20}, {X: 20, Y: 20}, {X: 20, Y: 40}, {X: 0, Y: 40}}
	cmds, err := GenerateVCarve(l, 0, testFeeds, VCarveParams{ToolAngleDeg: 90, ToolDiameter: 4, StepOver: 0.5})
	if err != nil {
		t.Fatalf("GenerateVCarve: %v", err)
	}
	// No cut point may be deeper than its true distance to the L boundary allows (depth ≤
	// distance/tan, i.e. the V-bit half-width never exceeds the distance to the nearest wall).
	for _, c := range cmds {
		x, hasX := c.Params["X"]
		y, hasY := c.Params["Y"]
		z, hasZ := c.Params["Z"]
		if c.Name != "G1" || !hasX || !hasY || !hasZ {
			continue
		}
		maxDepth := geom2d.DistanceToBoundary(geom2d.Point2{X: x, Y: y}, l) // tan45° = 1
		if -z > maxDepth+1e-6 {
			t.Errorf("point (%g,%g) cut to depth %g exceeds its distance-to-wall limit %g (gouge)", x, y, -z, maxDepth)
		}
	}
}

// TestVCarveErrors covers the degenerate tool/boundary cases.
func TestVCarveErrors(t *testing.T) {
	if _, err := GenerateVCarve(square(40), 0, testFeeds, VCarveParams{ToolDiameter: 0}); err == nil {
		t.Error("a zero tool diameter must error")
	}
	if _, err := GenerateVCarve(square(40)[:2], 0, testFeeds, VCarveParams{ToolDiameter: 4}); err == nil {
		t.Error("a degenerate boundary must error")
	}
}

// TestVCarveOuterContourMatchesBoundary checks the surface contour traces the boundary itself.
func TestVCarveOuterContourMatchesBoundary(t *testing.T) {
	cmds, err := GenerateVCarve(square(20), 0, testFeeds, VCarveParams{ToolDiameter: 4})
	if err != nil {
		t.Fatalf("GenerateVCarve: %v", err)
	}
	if a := cutPolygon(cmds).Area(); math.Abs(a-400) > 1 {
		t.Errorf("surface contour area = %g, want ~400 (the 20×20 boundary)", a)
	}
}
