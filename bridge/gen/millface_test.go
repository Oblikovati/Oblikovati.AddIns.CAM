// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"testing"
)

// TestMillFaceRaster checks the facing raster covers the inset region with alternating rows
// and respects the tool inset, and that an oversized tool errors.
func TestMillFaceRaster(t *testing.T) {
	cmds, err := GenerateMillFace(square(20), []float64{0}, testFeeds, MillFaceParams{ToolRadius: 2, StepOver: 0.5})
	if err != nil {
		t.Fatalf("GenerateMillFace: %v", err)
	}
	if countPlunges(cmds) != 1 {
		t.Errorf("one level → one plunge, got %d", countPlunges(cmds))
	}
	// All cutting moves stay within the radius-inset band [2,18] in X and Y.
	for _, c := range cmds {
		if c.Name != "G1" {
			continue
		}
		if x, ok := c.Params["X"]; ok && (x < 2-1e-9 || x > 18+1e-9) {
			t.Errorf("cut X=%g outside inset band [2,18]", x)
		}
	}
	// Several rows of back-and-forth cuts (more than a couple of X moves).
	xMoves := 0
	for _, c := range cmds {
		if c.Name == "G1" {
			if _, ok := c.Params["X"]; ok {
				xMoves++
			}
		}
	}
	if xMoves < 4 {
		t.Errorf("expected several raster rows, got %d X cuts", xMoves)
	}

	if _, err := GenerateMillFace(square(3), []float64{0}, testFeeds, MillFaceParams{ToolRadius: 2}); err == nil {
		t.Error("a tool too large for the region must error")
	}
}

// TestMillFaceSpiral checks the spiral facing pattern plunges only once per level (a continuous
// stay-down spiral, unlike the raster which also plunges once but links its rows differently),
// keeps every cut within the radius-inset band, and clears more than a single ring.
func TestMillFaceSpiral(t *testing.T) {
	cmds, err := GenerateMillFace(square(20), []float64{0}, testFeeds, MillFaceParams{ToolRadius: 2, StepOver: 0.5, Spiral: true})
	if err != nil {
		t.Fatalf("GenerateMillFace spiral: %v", err)
	}
	if countPlunges(cmds) != 1 {
		t.Errorf("a spiral level should plunge exactly once, got %d", countPlunges(cmds))
	}
	for _, c := range cmds {
		if c.Name != "G1" {
			continue
		}
		if x, ok := c.Params["X"]; ok && (x < 2-1e-9 || x > 18+1e-9) {
			t.Errorf("spiral cut X=%g outside inset band [2,18]", x)
		}
		if y, ok := c.Params["Y"]; ok && (y < 2-1e-9 || y > 18+1e-9) {
			t.Errorf("spiral cut Y=%g outside inset band [2,18]", y)
		}
	}
	if countCutMoves(cmds) < 8 {
		t.Errorf("the spiral should lay down several concentric rings, got %d cut moves", countCutMoves(cmds))
	}
}

// TestFaceSpiralRings checks the spiral rings begin with the inset rectangle and march inward.
func TestFaceSpiralRings(t *testing.T) {
	rect := insetRect(2, 2, 18, 18)
	rings := faceSpiralRings(rect, 3)
	if len(rings) < 2 {
		t.Fatalf("a 16×16 region at 3mm spacing should yield several rings, got %d", len(rings))
	}
	if rings[0].Area() <= rings[1].Area() {
		t.Errorf("rings must shrink inward: outer area %g, next %g", rings[0].Area(), rings[1].Area())
	}
	if rings := faceSpiralRings(rect, 0); len(rings) != 1 {
		t.Errorf("a non-positive spacing should yield just the outer ring, got %d", len(rings))
	}
}

// TestPassLines covers the spacing and the single-row fallback.
func TestPassLines(t *testing.T) {
	if rows := passLines(0, 1, 5); len(rows) != 1 || rows[0] != 0.5 {
		t.Errorf("thin band → single mid row, got %v", rows)
	}
	rows := passLines(0, 10, 2)
	if rows[0] != 0 || rows[len(rows)-1] != 10 {
		t.Errorf("rows must span the band ends, got %v", rows)
	}
	for i := 1; i < len(rows); i++ {
		if rows[i]-rows[i-1] > 2+1e-9 {
			t.Errorf("row spacing %g exceeds 2", rows[i]-rows[i-1])
		}
	}
}
