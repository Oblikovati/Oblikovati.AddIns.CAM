// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"math"
	"testing"
)

// TestVCarveDeepensInward checks the boundary contour is cut at the surface and inner contours
// progressively deeper, forming the V cross-section.
func TestVCarveDeepensInward(t *testing.T) {
	cmds, err := GenerateVCarve(square(40), 0, testFeeds, VCarveParams{ToolAngleDeg: 90, ToolDiameter: 4, StepOver: 0.5})
	if err != nil {
		t.Fatalf("GenerateVCarve: %v", err)
	}
	// collect the plunge depth of each contour (one per ring).
	var depths []float64
	for _, c := range cmds {
		if z, ok := c.Params["Z"]; ok && c.Name == "G1" {
			depths = append(depths, z)
		}
	}
	if len(depths) < 3 {
		t.Fatalf("expected several nested contours, got %d", len(depths))
	}
	// the first (boundary) contour is at the surface; each deeper.
	if !approx(depths[0], 0) {
		t.Errorf("boundary contour depth = %g, want 0 (surface)", depths[0])
	}
	for i := 1; i < len(depths); i++ {
		if depths[i] >= depths[i-1] {
			t.Errorf("contour %d depth %g is not deeper than %g", i, depths[i], depths[i-1])
		}
	}
	// a 90° V-bit cuts depth = offset distance; the second contour (offset 2mm) sits at z=-2.
	if !approx(depths[1], -2) {
		t.Errorf("second contour depth = %g, want -2 (2mm offset / tan45°)", depths[1])
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
