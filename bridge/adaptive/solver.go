// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import (
	"fmt"
	"math"

	"oblikovati.org/cam/bridge/clipper"
)

// minStepClipper is the solver's MIN_STEP_CLIPPER (16×3): the smallest tool step in scaled units,
// chosen so a finishing-pass-sized cut still measures a non-zero area after integer rounding (see
// the derivation in the upstream header). It also sets the scale factor.
const minStepClipper = 16.0 * 3

// finishingThicknessScale is the fraction of the stepover left for the finishing pass.
const finishingThicknessScale = 1.0 / 10.0

// solver holds the scaled-plane working state derived from a Config — the integer tool radius,
// stepover, helix-ramp radii, and the cut-area target the engagement loop steers toward. It is
// the Go analogue of an Adaptive2d instance. newSolver computes the pure scaling math; the
// engine-dependent reference geometry is built by buildToolGeometry.
type solver struct {
	cfg Config

	scaleFactor              int64
	toolRadiusScaled         int64
	stepOverScaled           float64
	helixRampMaxRadiusScaled int64
	helixRampMinRadiusScaled int64
	finishPassOffsetScaled   int64

	toolGeometry     clipper.Path // the tool disc at the origin (faceted), built by the engine
	referenceCutArea float64      // area of a half-radius-step slot cut (the engagement reference)
	optimalCutAreaPD float64      // target cut area per unit distance the loop holds
}

// newSolver derives the scaled working parameters from cfg. Tolerance is clamped to [0.01, 1.0];
// the scale factor is set so there are 1/tolerance minimum steps per stepover. Exact port of the
// initialisation block of Adaptive2d::Execute (pure arithmetic — no clipping engine needed).
func newSolver(cfg Config) *solver {
	tol := math.Max(cfg.Tolerance, 0.01)
	tol = math.Min(tol, 1.0)
	scaleFactor := int64(minStepClipper / tol / math.Min(1.0, cfg.StepOverFactor*cfg.ToolDiameter))

	toolRadiusScaled := int64(cfg.ToolDiameter * float64(scaleFactor) / 2)
	stepOverScaled := float64(toolRadiusScaled) * cfg.StepOverFactor

	target := cfg.HelixRampTargetDiameter
	if target < numericTolerance {
		target = cfg.ToolDiameter
	}
	target = math.Min(target, cfg.ToolDiameter)
	minDiameter := math.Max(cfg.HelixRampMinDiameter, cfg.ToolDiameter/8)
	target = math.Max(target, minDiameter)

	finishPassOffsetScaled := int64(0)
	if cfg.FinishingProfile {
		finishPassOffsetScaled = int64(stepOverScaled * finishingThicknessScale)
	}

	return &solver{
		cfg:                      cfg,
		scaleFactor:              scaleFactor,
		toolRadiusScaled:         toolRadiusScaled,
		stepOverScaled:           stepOverScaled,
		helixRampMaxRadiusScaled: int64(target * float64(scaleFactor) / 2),
		helixRampMinRadiusScaled: int64(minDiameter * float64(scaleFactor) / 2),
		finishPassOffsetScaled:   finishPassOffsetScaled,
	}
}

// buildToolGeometry builds the faceted tool disc and the engagement reference: the reference cut
// area is the material a slot cut removes when the tool steps over by half its radius, and the
// optimal cut area per distance scales that by the stepover factor. Needs the cgo clipping engine
// (offsets a point into a disc and subtracts a shifted copy); errors otherwise. Exact port.
func (s *solver) buildToolGeometry() error {
	disc, err := clipper.Offset(clipper.Paths{{{X: 0, Y: 0}}}, clipper.Round, clipper.OpenRound, float64(s.toolRadiusScaled), 0, 0)
	if err != nil {
		return fmt.Errorf("solver.buildToolGeometry disc: %w", err)
	}
	if len(disc) == 0 || len(disc[0]) == 0 {
		return fmt.Errorf("solver.buildToolGeometry: tool radius %d produced no disc", s.toolRadiusScaled)
	}
	s.toolGeometry = disc[0]

	slot := translatePath(disc[0], clipper.IntPoint{X: s.toolRadiusScaled / 2, Y: 0})
	crossing, err := clipper.Subtract(clipper.Paths{disc[0]}, clipper.Paths{slot})
	if err != nil {
		return fmt.Errorf("solver.buildToolGeometry slot: %w", err)
	}
	if len(crossing) == 0 {
		return fmt.Errorf("solver.buildToolGeometry: empty reference slot crossing")
	}
	s.referenceCutArea = math.Abs(clipper.Area(crossing[0]))
	s.optimalCutAreaPD = 2 * s.cfg.StepOverFactor * s.referenceCutArea / float64(s.toolRadiusScaled)
	return nil
}
