// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestFlatCutterProfile checks a flat endmill is a full-radius cylinder within its height and empty
// outside it.
func TestFlatCutterProfile(t *testing.T) {
	c := Cutter{Shape: CutterFlat, Radius: 3, Height: 10}
	if c.radiusAt(0) != 3 || c.radiusAt(9) != 3 {
		t.Errorf("flat body radius = %v/%v, want 3/3", c.radiusAt(0), c.radiusAt(9))
	}
	if c.radiusAt(-0.1) != 0 || c.radiusAt(11) != 0 {
		t.Errorf("flat out-of-height radius not zero")
	}
}

// TestBallCutterProfile checks the ball nose grows as a hemisphere from the tip then holds radius.
func TestBallCutterProfile(t *testing.T) {
	c := Cutter{Shape: CutterBall, Radius: 4, Height: 12}
	if c.radiusAt(0) != 0 { // the very tip is a point
		t.Errorf("ball tip radius = %v, want 0", c.radiusAt(0))
	}
	if math.Abs(c.radiusAt(4)-4) > 1e-9 { // at the ball centre height, full radius
		t.Errorf("ball equator radius = %v, want 4", c.radiusAt(4))
	}
	if mid := c.radiusAt(4 - 4*math.Sqrt2/2); math.Abs(mid-4*math.Sqrt2/2) > 1e-9 {
		t.Errorf("ball 45° radius = %v, want %v", mid, 4*math.Sqrt2/2)
	}
	if c.radiusAt(8) != 4 { // above the ball, cylindrical shaft
		t.Errorf("ball shaft radius = %v, want 4", c.radiusAt(8))
	}
}

// TestConeCutterProfile checks a v/drill cone grows linearly then caps at the full radius.
func TestConeCutterProfile(t *testing.T) {
	c := Cutter{Shape: CutterCone, Radius: 5, Height: 20, TanHalf: 1}
	if c.radiusAt(3) != 3 {
		t.Errorf("cone radius at dz=3 = %v, want 3", c.radiusAt(3))
	}
	if c.radiusAt(10) != 5 { // capped at full radius
		t.Errorf("cone radius at dz=10 = %v, want 5 (capped)", c.radiusAt(10))
	}
}

// TestCutterFromTool maps tool shapes to cutter profiles, defaulting a zero cutting height to the
// fallback (the stock height) so the cutter always spans the cut.
func TestCutterFromTool(t *testing.T) {
	flat := CutterFromTool(ToolBit{ShapeType: "endmill", Diameter: 6, CuttingEdgeHeight: 0}, 30)
	if flat.Shape != CutterFlat || flat.Radius != 3 || flat.Height != 30 {
		t.Errorf("endmill cutter = %+v", flat)
	}
	ball := CutterFromTool(ToolBit{ShapeType: "ballend", Diameter: 8, CuttingEdgeHeight: 12}, 30)
	if ball.Shape != CutterBall || ball.Radius != 4 || ball.Height != 12 {
		t.Errorf("ballend cutter = %+v", ball)
	}
	drill := CutterFromTool(ToolBit{ShapeType: "drill", Diameter: 6}, 30)
	if drill.Shape != CutterCone || drill.TanHalf <= 0 {
		t.Errorf("drill cutter = %+v", drill)
	}
}
