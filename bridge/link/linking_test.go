// SPDX-License-Identifier: GPL-2.0-only

package link

import (
	"errors"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// fakeProbe is a named CollisionProbe stand-in: it returns whatever clearance (mm) its function
// yields for the probed segment, or a fixed error. Tests drive the keep-tool-down decision by
// modelling the part's reach through this function rather than calling a host.
type fakeProbe struct {
	clearance func(pts []gcode.Vector3, toolRadius float64) float64
	err       error
}

func (f fakeProbe) PartClearance(pts []gcode.Vector3, toolRadius float64) (float64, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.clearance(pts, toolRadius), nil
}

// clearAbove models a part whose top sits at z=topZ: a traverse clears it by (segmentZ - topZ),
// clamped at 0, so low traverses collide and high ones clear.
func clearAbove(topZ float64) func([]gcode.Vector3, float64) float64 {
	return func(pts []gcode.Vector3, _ float64) float64 {
		d := pts[0].Z - topZ
		if d < 0 {
			return 0
		}
		return d
	}
}

func TestCheckCollisionCoincidentNeverCollides(t *testing.T) {
	called := false
	probe := fakeProbe{clearance: func([]gcode.Vector3, float64) float64 { called = true; return 0 }}
	hit, err := CheckCollision(gcode.Vector3{X: 1, Y: 2, Z: 3}, gcode.Vector3{X: 1, Y: 2, Z: 3}, probe, 0, 1)
	if err != nil || hit {
		t.Fatalf("coincident move: hit=%v err=%v, want false/nil", hit, err)
	}
	if called {
		t.Error("a coincident move must not probe the part")
	}
}

func TestCheckCollisionThresholds(t *testing.T) {
	clear := fakeProbe{clearance: func([]gcode.Vector3, float64) float64 { return 5 }}
	if hit, _ := CheckCollision(gcode.Vector3{}, gcode.Vector3{X: 10}, clear, 0, 1); hit {
		t.Error("5 mm clearance vs 1 mm threshold should not collide")
	}
	blocked := fakeProbe{clearance: func([]gcode.Vector3, float64) float64 { return 0.5 }}
	if hit, _ := CheckCollision(gcode.Vector3{}, gcode.Vector3{X: 10}, blocked, 0, 1); !hit {
		t.Error("0.5 mm clearance vs 1 mm threshold should collide")
	}
}

func TestGetLinkingMovesKeepsToolDown(t *testing.T) {
	// The straight move clears the part everywhere, so the lowest candidate (the cut depth) wins:
	// one horizontal traverse at depth, no lift.
	probe := fakeProbe{clearance: func([]gcode.Vector3, float64) float64 { return 10 }}
	moves, err := GetLinkingMoves(gcode.Vector3{}, gcode.Vector3{X: 10}, []float64{0, 5, 20}, probe, 0, 1)
	if err != nil {
		t.Fatalf("GetLinkingMoves: %v", err)
	}
	if len(moves) != 1 {
		t.Fatalf("keep-tool-down should be a single traverse, got %d moves: %v", len(moves), moves)
	}
	if moves[0].Params["Z"] != 0 || moves[0].Params["X"] != 10 {
		t.Errorf("traverse = %v, want a direct move to X10 at Z0", moves[0].Params)
	}
}

func TestGetLinkingMovesLiftsWhenBlocked(t *testing.T) {
	// The part top is at z=5, so the depth-0 and depth-5 traverses collide and the link lifts to
	// the next candidate height (20), traverses across, then plunges back down in steps through the
	// previously-tried plane (5) to the target depth (0) — a lift, a traverse, two plunge legs.
	probe := fakeProbe{clearance: clearAbove(5)}
	moves, err := GetLinkingMoves(gcode.Vector3{}, gcode.Vector3{X: 10}, []float64{0, 5, 20}, probe, 0, 1)
	if err != nil {
		t.Fatalf("GetLinkingMoves: %v", err)
	}
	if len(moves) != 4 {
		t.Fatalf("blocked link should lift, traverse, then step-plunge (4 moves), got %d: %v", len(moves), moves)
	}
	if moves[0].Params["Z"] != 20 || !roughlyEqual(moves[0].Params["X"], 0) {
		t.Errorf("first move = %v, want a vertical lift to Z20", moves[0].Params)
	}
	// The traverse holds Z modally (Z is not re-emitted) and moves across in X.
	if moves[1].Params["X"] != 10 {
		t.Errorf("second move = %v, want the traverse across to X10", moves[1].Params)
	}
	if _, hasZ := moves[1].Params["Z"]; hasZ {
		t.Errorf("traverse should not repeat the modal Z, got %v", moves[1].Params)
	}
	if last := moves[len(moves)-1]; last.Params["Z"] != 0 {
		t.Errorf("last move = %v, want a plunge back to Z0", last.Params)
	}
}

func TestGetLinkingMovesNoClearLink(t *testing.T) {
	probe := fakeProbe{clearance: func([]gcode.Vector3, float64) float64 { return 0 }}
	if _, err := GetLinkingMoves(gcode.Vector3{}, gcode.Vector3{X: 10}, []float64{0, 5}, probe, 0, 1); !errors.Is(err, ErrNoClearLink) {
		t.Errorf("all-blocked link err = %v, want ErrNoClearLink", err)
	}
}

func TestGetLinkingMovesCoincidentIsEmpty(t *testing.T) {
	probe := fakeProbe{clearance: func([]gcode.Vector3, float64) float64 { return 0 }}
	moves, err := GetLinkingMoves(gcode.Vector3{X: 4}, gcode.Vector3{X: 4}, []float64{0, 5}, probe, 0, 1)
	if err != nil || moves != nil {
		t.Errorf("coincident link = %v, %v, want nil/nil", moves, err)
	}
}

func TestGetLinkingMovesPropagatesProbeError(t *testing.T) {
	probe := fakeProbe{err: errors.New("host down")}
	if _, err := GetLinkingMoves(gcode.Vector3{}, gcode.Vector3{X: 10}, []float64{0}, probe, 0, 1); err == nil {
		t.Error("a probe error must surface, not be swallowed as no-link")
	}
}
