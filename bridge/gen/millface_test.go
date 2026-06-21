// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// cutBandViolations reports cutting moves whose X or Y leaves the radius-inset band [lo,hi].
func cutBandViolations(cmds []gcode.Command, lo, hi float64) int {
	bad := 0
	for _, c := range cmds {
		if c.Name != "G1" {
			continue
		}
		if x, ok := c.Params["X"]; ok && (x < lo-1e-6 || x > hi+1e-6) {
			bad++
		}
		if y, ok := c.Params["Y"]; ok && (y < lo-1e-6 || y > hi+1e-6) {
			bad++
		}
	}
	return bad
}

// TestMillFaceRaster checks the default zigzag facing covers the inset region with alternating rows
// linked at depth (one plunge), respects the tool inset, and that an oversized tool errors.
func TestMillFaceRaster(t *testing.T) {
	cmds, err := GenerateMillFace(square(20), []float64{0}, testFeeds, MillFaceParams{ToolRadius: 2, StepOver: 0.5})
	if err != nil {
		t.Fatalf("GenerateMillFace: %v", err)
	}
	if countPlunges(cmds) != 1 {
		t.Errorf("zigzag links its rows at depth → one plunge, got %d", countPlunges(cmds))
	}
	if n := cutBandViolations(cmds, 2, 18); n != 0 {
		t.Errorf("%d cutting moves left the radius-inset band [2,18]", n)
	}
	dirs := rowDirections(cmds)
	sawPlus, sawMinus := false, false
	for _, d := range dirs {
		sawPlus = sawPlus || d > 0
		sawMinus = sawMinus || d < 0
	}
	if !sawPlus || !sawMinus {
		t.Errorf("a zigzag should sweep both directions, got %v", dirs)
	}

	if _, err := GenerateMillFace(square(3), []float64{0}, testFeeds, MillFaceParams{ToolRadius: 2}); err == nil {
		t.Error("a tool too large for the region must error")
	}
}

// TestMillFaceDirectional checks the one-way (directional) pattern cuts every row in the same
// direction (consistent climb) and lifts/repositions between rows rather than linking at depth.
func TestMillFaceDirectional(t *testing.T) {
	cmds, err := GenerateMillFace(square(20), []float64{0}, testFeeds, MillFaceParams{
		ToolRadius: 2, StepOver: 0.5, Pattern: FacePatternDirectional,
	})
	if err != nil {
		t.Fatalf("directional facing: %v", err)
	}
	if cutBandViolations(cmds, 2, 18) != 0 {
		t.Error("directional cuts must stay in the inset band [2,18]")
	}
	for _, d := range rowDirections(cmds) {
		if d < 0 {
			t.Errorf("one-way facing must never reverse a row, got dirs %v", rowDirections(cmds))
			break
		}
	}
	if countPlunges(cmds) <= 1 {
		t.Errorf("directional lifts and re-plunges per row, want several plunges, got %d", countPlunges(cmds))
	}
}

// TestMillFaceBidirectional checks the bidirectional pattern alternates direction (like zigzag) but
// lifts between rows (more than one plunge, unlike the linked zigzag).
func TestMillFaceBidirectional(t *testing.T) {
	zig, err := GenerateMillFace(square(20), []float64{0}, testFeeds, MillFaceParams{ToolRadius: 2, StepOver: 0.5})
	if err != nil {
		t.Fatalf("zigzag facing: %v", err)
	}
	bidi, err := GenerateMillFace(square(20), []float64{0}, testFeeds, MillFaceParams{
		ToolRadius: 2, StepOver: 0.5, Pattern: FacePatternBidirectional,
	})
	if err != nil {
		t.Fatalf("bidirectional facing: %v", err)
	}
	sawPlus, sawMinus := false, false
	for _, d := range rowDirections(bidi) {
		sawPlus = sawPlus || d > 0
		sawMinus = sawMinus || d < 0
	}
	if !sawPlus || !sawMinus {
		t.Error("bidirectional should alternate direction like a zigzag")
	}
	if countPlunges(bidi) <= countPlunges(zig) {
		t.Errorf("bidirectional lifts between rows: plunges %d should exceed zigzag's %d", countPlunges(bidi), countPlunges(zig))
	}
}

// TestMillFaceAngle checks an angled raster still clears the region within the inset band: at 90° the
// rows run along Y instead of X, and every cut stays inside [2,18].
func TestMillFaceAngle(t *testing.T) {
	for _, angle := range []float64{45, 90} {
		cmds, err := GenerateMillFace(square(20), []float64{0}, testFeeds, MillFaceParams{
			ToolRadius: 2, StepOver: 0.5, Angle: angle,
		})
		if err != nil {
			t.Fatalf("angled facing %g°: %v", angle, err)
		}
		if cutBandViolations(cmds, 2, 18) != 0 {
			t.Errorf("angled (%g°) cuts must stay in the inset band [2,18]", angle)
		}
		if countCutMoves(cmds) < 4 {
			t.Errorf("angled (%g°) facing should lay several rows, got %d cut moves", angle, countCutMoves(cmds))
		}
	}
}

// TestMillFaceSpiral checks the spiral facing pattern plunges only once per level, keeps every cut
// within the radius-inset band, and clears more than a single ring.
func TestMillFaceSpiral(t *testing.T) {
	cmds, err := GenerateMillFace(square(20), []float64{0}, testFeeds, MillFaceParams{ToolRadius: 2, StepOver: 0.5, Spiral: true})
	if err != nil {
		t.Fatalf("GenerateMillFace spiral: %v", err)
	}
	if countPlunges(cmds) != 1 {
		t.Errorf("a spiral level should plunge exactly once, got %d", countPlunges(cmds))
	}
	if cutBandViolations(cmds, 2, 18) != 0 {
		t.Error("spiral cuts must stay in the inset band [2,18]")
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
