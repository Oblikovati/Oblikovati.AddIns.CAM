// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"math"
	"sort"

	"oblikovati.org/api/wire"
	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
)

// DrillTarget is one hole to drill: the hole-axis XY and the top (start) and bottom (end)
// Z, all in millimetres. The Z axis is assumed vertical (milestone-1 drilling handles
// Z-aligned holes; tilted holes need a work-plane transform, a later milestone).
type DrillTarget struct {
	X, Y   float64
	Top    float64
	Bottom float64
}

// DrillingOp drills a set of circular holes with a canned cycle. It ports FreeCAD's
// Path/Op/Drilling (CircularHoleBase): the holes are resolved from the part's cylindrical
// faces, sorted, and each emitted as a G81/G82/G83/G85 cycle via the drill generator. The
// peck/dwell/feed-retract knobs map straight onto gen.DrillParams.
type DrillingOp struct {
	OpBase
	DwellTime   float64 // G82 dwell (s); >0 selects dwell
	PeckDepth   float64 // G83/G73 peck increment (mm); >0 selects peck
	ChipBreak   bool    // G73 instead of G83
	FeedRetract bool    // G85 (boring/reaming)
	Repeat      int     // L repeat count (<=1 omits L)
	Holes       []DrillTarget
}

// Features reports the property groups drilling uses (a tool, depths, heights, and the
// cylindrical-face geometry it is driven by — but no multi-pass step-down: a canned cycle
// drills full depth in one command).
func (op *DrillingOp) Features() FeatureFlag {
	return FeatureTool | FeatureDepths | FeatureHeights | FeatureBaseGeometry
}

// Execute generates the drilling toolpath: rapid to the clearance plane, then for each
// hole rapid over its XY and emit the canned cycle (carrying the plunge feed from the tool
// controller), then cancel the cycle with G80. The standard label/clearance framing is
// applied by OpBase.frame. Returns an error if the tool controller is missing or any
// hole's geometry is illegal (e.g. bottom above top).
func (op *DrillingOp) Execute(job *Job) (gcode.Path, error) {
	tc, err := op.resolveTool(job)
	if err != nil {
		return gcode.Path{}, err
	}
	if len(op.Holes) == 0 {
		return gcode.Path{}, fmt.Errorf("drilling operation %q has no holes to drill", op.OpLabel)
	}

	holes := sortedHoles(op.Holes)
	repeat := op.Repeat
	if repeat < 1 {
		repeat = 1
	}

	cutting := []gcode.Command{gcode.NewCommand("G0", map[string]float64{"Z": op.ClearanceHeight})}
	for _, h := range holes {
		retract := op.RetractHeight
		params := gen.DrillParams{
			DwellTime:     op.DwellTime,
			PeckDepth:     op.PeckDepth,
			Repeat:        repeat,
			RetractHeight: &retract,
			ChipBreak:     op.ChipBreak,
			FeedRetract:   op.FeedRetract,
		}
		cmds, err := gen.GenerateDrill(
			gcode.Vector3{X: h.X, Y: h.Y, Z: h.Top},
			gcode.Vector3{X: h.X, Y: h.Y, Z: h.Bottom},
			params,
		)
		if err != nil {
			return gcode.Path{}, fmt.Errorf("drilling operation %q, hole at (%g,%g): %w", op.OpLabel, h.X, h.Y, err)
		}
		// The canned cycle plunges at the tool controller's vertical feed.
		cmds[0].Params["F"] = tc.VertFeed
		// Rapid over the hole, then run the cycle.
		cutting = append(cutting, gcode.NewCommand("G0", map[string]float64{"X": h.X, "Y": h.Y}))
		cutting = append(cutting, cmds...)
	}
	cutting = append(cutting, gcode.NewCommand("G80", nil)) // cancel canned cycle

	return op.frame(cutting), nil
}

// sortedHoles returns the holes in a deterministic drilling order — by Y then X — so the
// generated program is stable across runs (a proper nearest-neighbour/TSP ordering is a
// later optimisation; FreeCAD's sort_locations does the same job).
func sortedHoles(holes []DrillTarget) []DrillTarget {
	out := append([]DrillTarget(nil), holes...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Y != out[j].Y {
			return out[i].Y < out[j].Y
		}
		return out[i].X < out[j].X
	})
	return out
}

// holeDedupTol is the XY distance (mm) within which two cylindrical faces are treated as
// the same hole (a counterbore/through-hole presents several coaxial cylinder faces).
const holeDedupTol = 1e-3

// DetectDrillTargets derives drill targets from a body's topology: every cylindrical face
// becomes a hole at its axis XY (the face's representative point), drilled from the body's
// top Z to its bottom Z. Inputs are the host's ReferenceKeys (face classification +
// representative points) and RangeBox (extent), both in centimetres; outputs are
// millimetres. Coaxial faces are de-duplicated.
//
// Milestone-1 assumptions, documented so callers know the envelope: holes are vertical
// (Z-aligned) and through (top/bottom = body extent). Blind-hole depth and tilted axes
// require per-face range boxes / work-plane transforms — later milestones.
func DetectDrillTargets(refs wire.ReferenceKeysResult, rbox wire.BodyRangeBoxResult, bodyIndex int) ([]DrillTarget, error) {
	if bodyIndex < 0 || bodyIndex >= len(refs.Bodies) {
		return nil, fmt.Errorf("body index %d out of range (%d bodies)", bodyIndex, len(refs.Bodies))
	}
	if len(rbox.Min) < 3 || len(rbox.Max) < 3 {
		return nil, fmt.Errorf("range box has no axis-aligned extent (min=%v max=%v)", rbox.Min, rbox.Max)
	}
	top := rbox.Max[2] * cmToMM
	bottom := rbox.Min[2] * cmToMM

	var targets []DrillTarget
	for _, face := range refs.Bodies[bodyIndex].Faces {
		if face.Kind != "cylinder" || len(face.Point) < 3 {
			continue
		}
		t := DrillTarget{X: face.Point[0] * cmToMM, Y: face.Point[1] * cmToMM, Top: top, Bottom: bottom}
		if !containsHole(targets, t) {
			targets = append(targets, t)
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("body %d has no cylindrical faces to drill", bodyIndex)
	}
	return sortedHoles(targets), nil
}

// containsHole reports whether a coaxial hole is already present within holeDedupTol.
func containsHole(targets []DrillTarget, t DrillTarget) bool {
	for _, e := range targets {
		if math.Hypot(e.X-t.X, e.Y-t.Y) <= holeDedupTol {
			return true
		}
	}
	return false
}
