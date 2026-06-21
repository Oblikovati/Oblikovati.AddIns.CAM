// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

package adaptive

import (
	"testing"

	"oblikovati.org/cam/bridge/clipper"
)

// squareDPath returns a square contour (mm) of the given side centred at the origin's positive
// quadrant from (0,0) to (side,side).
func squareDPath(side float64) DPath {
	return DPath{{X: 0, Y: 0}, {X: side, Y: 0}, {X: side, Y: side}, {X: 0, Y: side}}
}

func TestExecuteClearsASquarePocket(t *testing.T) {
	cfg := DefaultConfig() // 5mm tool, clearing inside, finishing on
	// A 30mm square pocket; stock is the same square.
	pocket := squareDPath(30)
	outputs, err := Execute(cfg, []DPath{pocket}, []DPath{pocket}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(outputs) != 1 {
		t.Fatalf("a single square pocket should yield one region, got %d", len(outputs))
	}
	out := outputs[0]
	if out.StartPointNotFound {
		t.Fatal("the pocket should have been entered")
	}
	if len(out.AdaptivePaths) == 0 {
		t.Fatal("Execute should produce toolpaths")
	}
	// The helix centre / start point must lie inside the pocket.
	if out.StartPoint.X <= 0 || out.StartPoint.X >= 30 || out.StartPoint.Y <= 0 || out.StartPoint.Y >= 30 {
		t.Fatalf("start point %v not inside the 30mm pocket", out.StartPoint)
	}
	hasCut := false
	for _, tp := range out.AdaptivePaths {
		if tp.Motion == MotionCutting && len(tp.Pts) >= 2 {
			hasCut = true
		}
	}
	if !hasCut {
		t.Fatal("expected engaged cutting toolpaths")
	}
}

func TestExecuteSkipsFullyClearedPocket(t *testing.T) {
	cfg := DefaultConfig()
	pocket := squareDPath(20)
	// Mark the whole pocket as already cleared: there is nothing left to do.
	outputs, err := Execute(cfg, []DPath{pocket}, []DPath{pocket}, []DPath{squareDPath(20)})
	if err != nil {
		t.Fatal(err)
	}
	if len(outputs) != 0 {
		t.Fatalf("an already-cleared pocket should produce no regions, got %d", len(outputs))
	}
}

func TestExecuteRejectsProfiling(t *testing.T) {
	cfg := DefaultConfig()
	cfg.OpType = ProfilingInside
	if _, err := Execute(cfg, nil, []DPath{squareDPath(10)}, nil); err == nil {
		t.Fatal("profiling op types should be rejected (clearing only)")
	}
}

func TestFixOrientationMakesExteriorPositive(t *testing.T) {
	// A clockwise (negative) exterior square should be flipped to positive.
	cw := clipper.Path{{X: 0, Y: 0}, {X: 0, Y: 100}, {X: 100, Y: 100}, {X: 100, Y: 0}}
	paths := clipper.Paths{cw}
	fixOrientation(paths)
	if !clipper.Orientation(paths[0]) {
		t.Fatal("an exterior boundary should wind positive after fixOrientation")
	}
}

func TestPerCurveOffsetShrinksExterior(t *testing.T) {
	sq := clipper.Path{{X: 0, Y: 0}, {X: 1000, Y: 0}, {X: 1000, Y: 1000}, {X: 0, Y: 1000}}
	paths := clipper.Paths{sq}
	// delta -100 with exterior direction +1 shrinks the square.
	out, err := perCurveOffset(paths, paths, -100)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("offset of one square should give one path, got %d", len(out))
	}
	before := clipper.Area(sq)
	after := clipper.Area(out[0])
	if after >= before || after <= 0 {
		t.Fatalf("exterior offset should shrink the area: before %g after %g", before, after)
	}
}
