// SPDX-License-Identifier: GPL-2.0-only

// Package adaptive computes adaptive high-speed-machining clearing toolpaths: it steers the
// tool to hold a near-constant radial engagement (a target cut area) against a model of the
// already-cleared region, so the controller can hold a high feed and corners do not overload
// the tool. This is a faithful Go port of the Adaptive2d solver, built on the integer
// polygon-clipping engine in bridge/clipper (the heavyweight boolean/offset runs through cgo;
// the per-point engagement math is pure Go).
//
// The solver works internally in a SCALED INTEGER plane (real millimetres × a scale factor) so
// the clipping arithmetic is exact; inputs and Output are in millimetres and are scaled in/out
// at the boundary.
package adaptive

// MotionType tags each emitted sub-path with how the tool moves along it. The values match the
// solver's enum so they serialise unchanged.
type MotionType int

const (
	MotionCutting           MotionType = iota // engaged cutting move
	MotionLinkClear                           // rapid/link over already-cleared area
	MotionLinkNotClear                        // link that may still touch uncut stock
	MotionLinkClearAtPrevPass                 // unused; kept for value parity
)

// OperationType selects what the solver clears: the inside or the outside of the driving
// region, as a clearing (area) or a profiling (boundary-following) operation.
type OperationType int

const (
	ClearingInside OperationType = iota
	ClearingOutside
	ProfilingInside
	ProfilingOutside
)

// DoublePoint is a point in real millimetre space (the unscaled output domain). The solver's
// internal geometry uses clipper.IntPoint in the scaled plane.
type DoublePoint struct{ X, Y float64 }

// DPath is an output toolpath in millimetres.
type DPath []DoublePoint

// TPath is one motion-tagged output sub-path.
type TPath struct {
	Motion MotionType
	Pts    DPath
}

// Output is the result of clearing one connected region: the helix entry, the start point, the
// ordered toolpath segments, and the warning flags the solver raises when it has to compromise
// (so the caller can surface "uncleared area remains", "too many failed engagements", etc.).
type Output struct {
	HelixCenter   DoublePoint
	StartPoint    DoublePoint
	AdaptivePaths []TPath
	ReturnMotion  MotionType
	ClearedArea   float64 // total area cleared in this region (scaled-plane units)

	StartPointNotFound         bool
	LeadPathFailed             bool
	UnexpectedRotateIterations bool
	TooManyFailedEngagements   bool
	UnclearedAreaRemains       bool
	FailedToSetUpFinishingPass bool
	FinishingLeadInFailed      bool
}

// Config holds the public knobs of the adaptive solver. Zero values are NOT valid; build one
// with DefaultConfig and override. The defaults mirror the Adaptive2d member initialisers.
type Config struct {
	ToolDiameter            float64 // tool diameter (mm)
	HelixRampTargetDiameter float64 // target helix-entry diameter (mm); 0 → tool diameter
	HelixRampMinDiameter    float64 // minimum helix-entry diameter (mm); 0 → toolDiameter/8
	StepOverFactor          float64 // target radial engagement as a fraction of the tool radius
	Tolerance               float64 // step resolution (mm-ish); clamped to [0.01, 1.0]
	StockToLeave            float64 // offset left on the region walls (mm)
	ForceInsideOut          bool    // clear from the inside outward
	FinishingProfile        bool    // add a thin finishing pass along the walls
	KeepToolDownDistRatio   float64 // keep the tool down when a link is within this × stepover
	OpType                  OperationType
}

// DefaultConfig returns the solver defaults (a 5 mm tool, 20% radial engagement, inside-out
// clearing with a finishing profile) — the same starting point as the upstream solver.
func DefaultConfig() Config {
	return Config{
		ToolDiameter:          5,
		StepOverFactor:        0.2,
		Tolerance:             0.1,
		ForceInsideOut:        true,
		FinishingProfile:      true,
		KeepToolDownDistRatio: 3.0,
		OpType:                ClearingInside,
	}
}

// numericTolerance is the solver's NTOL: distances below it are treated as zero.
const numericTolerance = 1.0e-7
