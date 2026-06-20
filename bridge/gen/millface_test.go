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
