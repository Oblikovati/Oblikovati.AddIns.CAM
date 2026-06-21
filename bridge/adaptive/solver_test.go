// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import (
	"math"
	"testing"
)

func TestNewSolverScaling(t *testing.T) {
	// Defaults: 5 mm tool, 20% engagement, 0.1 tolerance.
	//   scaleFactor      = 48 / 0.1 / min(1, 0.2·5) = 480
	//   toolRadiusScaled = 5·480/2 = 1200, stepOverScaled = 1200·0.2 = 240
	//   helix max radius = 5·480/2 = 1200 (target falls back to tool diameter)
	//   helix min radius = (5/8)·480/2 = 150
	//   finishPassOffset = 240·0.1 = 24
	s := newSolver(DefaultConfig())
	checks := []struct {
		name string
		got  int64
		want int64
	}{
		{"scaleFactor", s.scaleFactor, 480},
		{"toolRadiusScaled", s.toolRadiusScaled, 1200},
		{"helixRampMaxRadiusScaled", s.helixRampMaxRadiusScaled, 1200},
		{"helixRampMinRadiusScaled", s.helixRampMinRadiusScaled, 150},
		{"finishPassOffsetScaled", s.finishPassOffsetScaled, 24},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
	if s.stepOverScaled != 240 {
		t.Errorf("stepOverScaled = %g, want 240", s.stepOverScaled)
	}
}

func TestNewSolverToleranceClamped(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Tolerance = 0 // below the 0.01 floor → clamped, so scaleFactor stays finite
	if s := newSolver(cfg); s.scaleFactor != int64(minStepClipper/0.01/1.0) {
		t.Fatalf("tolerance not clamped to 0.01: scaleFactor = %d", s.scaleFactor)
	}
	cfg.Tolerance = 5 // above the 1.0 ceiling → clamped to 1.0
	if s := newSolver(cfg); s.scaleFactor != int64(minStepClipper/1.0/1.0) {
		t.Fatalf("tolerance not clamped to 1.0: scaleFactor = %d", s.scaleFactor)
	}
}

func TestNewSolverNoFinishPass(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FinishingProfile = false
	if s := newSolver(cfg); s.finishPassOffsetScaled != 0 {
		t.Fatalf("finishing profile off should give 0 offset, got %d", s.finishPassOffsetScaled)
	}
}

func TestBuildToolGeometry(t *testing.T) {
	s := newSolver(DefaultConfig())
	err := s.buildToolGeometry()
	if !engineAvailable() {
		if err == nil {
			t.Fatal("buildToolGeometry should error without the cgo engine")
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(s.toolGeometry) < 8 {
		t.Fatalf("tool disc should be a many-sided polygon, got %d vertices", len(s.toolGeometry))
	}
	// The reference cut area is a half-radius-step slot: the crescent for d = r/2, within ~2% of
	// the analytic value (the disc is faceted).
	want := crescentArea(float64(s.toolRadiusScaled), float64(s.toolRadiusScaled)/2)
	if rel := math.Abs(s.referenceCutArea-want) / want; rel > 0.02 {
		t.Fatalf("referenceCutArea = %g, want ~%g (%.1f%% off)", s.referenceCutArea, want, rel*100)
	}
	wantPD := 2 * s.cfg.StepOverFactor * s.referenceCutArea / float64(s.toolRadiusScaled)
	if math.Abs(s.optimalCutAreaPD-wantPD) > 1e-9 {
		t.Fatalf("optimalCutAreaPD = %g, want %g", s.optimalCutAreaPD, wantPD)
	}
}
