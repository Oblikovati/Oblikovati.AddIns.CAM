// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

package gen

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// minCutZ returns the deepest Z among the feed moves, and whether any feed move was seen.
func minCutZ(cmds []gcode.Command) (float64, bool) {
	minZ, seen := math.Inf(1), false
	for _, c := range cmds {
		if c.Name != "G1" {
			continue
		}
		if z, ok := c.Params["Z"]; ok {
			if z < minZ {
				minZ = z
			}
			seen = true
		}
	}
	return minZ, seen
}

// TestVCarveCentreDepthEqualsClearance is the medial-axis depth oracle: on a 40mm square the medial
// axis runs through the centre, where the largest inscribed circle has radius 20mm. With a 90° bit
// (scale 1) big enough not to bottom out, the deepest carved point is therefore z = -20 — the
// clearance times the scale.
func TestVCarveCentreDepthEqualsClearance(t *testing.T) {
	cmds, err := GenerateVCarve(square(40), 0, testFeeds, VCarveParams{
		ToolAngleDeg: 90, ToolDiameter: 40, FinalDepth: -100,
	})
	if err != nil {
		t.Fatalf("GenerateVCarve: %v", err)
	}
	z, ok := minCutZ(cmds)
	if !ok {
		t.Fatal("v-carve produced no cutting moves")
	}
	if math.Abs(z-(-20)) > 1e-3 {
		t.Errorf("deepest carve = %.4f, want -20 (the centre clearance of a 40mm square)", z)
	}
}

// TestVCarveClampsToToolDepth checks the bit's reach limits the carve: a 4mm 90° bit can only reach
// rMax·scale = 2mm deep, so no point is cut past z = -2 however wide the region, and the depth still
// varies (the walls are cut shallower than the spine).
func TestVCarveClampsToToolDepth(t *testing.T) {
	cmds, err := GenerateVCarve(square(40), 0, testFeeds, VCarveParams{
		ToolAngleDeg: 90, ToolDiameter: 4, FinalDepth: -100,
	})
	if err != nil {
		t.Fatalf("GenerateVCarve: %v", err)
	}
	deepest, ok := minCutZ(cmds)
	if !ok {
		t.Fatal("v-carve produced no cutting moves")
	}
	if math.Abs(deepest-(-2)) > 1e-3 {
		t.Errorf("deepest carve = %.4f, want -2 (the 4mm bit bottoms out at rMax·scale)", deepest)
	}
	// Depth must vary: some feed move is shallower than the floor (the walls are not at full depth).
	shallow := false
	for _, c := range cmds {
		if c.Name == "G1" {
			if z, ok := c.Params["Z"]; ok && z > deepest+1e-6 {
				shallow = true
				break
			}
		}
	}
	if !shallow {
		t.Error("v-carve depth should vary from the walls to the spine, not be a single plane")
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
