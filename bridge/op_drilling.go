// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"math"

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
// faces, ordered into a short travel tour, and each emitted as a G81/G82/G83/G85 cycle via the
// drill generator. The
// peck/dwell/feed-retract knobs map straight onto gen.DrillParams.
type DrillingOp struct {
	OpBase
	DwellTime   float64 // G82 dwell (s); >0 selects dwell
	PeckDepth   float64 // G83/G73 peck increment (mm); >0 selects peck
	ChipBreak   bool    // G73 instead of G83
	FeedRetract bool    // G85 (boring/reaming)
	Repeat      int     // L repeat count (<=1 omits L)
	Depth       float64 // blind-hole depth below each hole top (mm); 0 → drill through to the hole bottom
	RetractToR  bool    // true → G99 (retract to the R plane between holes, fast); false → G98 (retract to clearance, clears clamps)
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

	holes := orderedHoles(op.Holes)
	repeat := op.Repeat
	if repeat < 1 {
		repeat = 1
	}

	cutting := []gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"Z": op.ClearanceHeight}),
		gcode.NewCommand(retractMode(op.RetractToR), nil), // canned-cycle retract mode (modal)
	}
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
		bottom := h.Bottom
		if op.Depth > 0 { // a set drill depth makes a blind hole: stop short of the through bottom
			bottom = h.Top - op.Depth
		}
		cmds, err := gen.GenerateDrill(
			gcode.Vector3{X: h.X, Y: h.Y, Z: h.Top},
			gcode.Vector3{X: h.X, Y: h.Y, Z: bottom},
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

// retractMode returns the canned-cycle retract G-code: G99 (return to the R plane between holes)
// when retractToR is set, else G98 (return to the initial clearance plane, which clears clamps).
func retractMode(retractToR bool) string {
	if retractToR {
		return "G99"
	}
	return "G98"
}

// orderedHoles returns the holes in a short, deterministic drilling order: a nearest-neighbour
// tour that starts at the lowest hole (min Y, then min X) and repeatedly hops to the closest
// remaining one, cutting the rapid travel between holes versus a plain row-by-row sort. Starting
// from a fixed anchor and breaking distance ties by (Y, X) keeps the tour stable across runs.
// FreeCAD's sort_locations does the same job. Replaces the earlier Y-then-X sort.
func orderedHoles(holes []DrillTarget) []DrillTarget {
	remaining := append([]DrillTarget(nil), holes...)
	if len(remaining) < 2 {
		return remaining
	}
	tour := make([]DrillTarget, 0, len(remaining))
	tour = append(tour, takeAt(&remaining, anchorHole(remaining)))
	for len(remaining) > 0 {
		tour = append(tour, takeAt(&remaining, nearestHole(tour[len(tour)-1], remaining)))
	}
	return tour
}

// anchorHole returns the index of the tour's start: the lowest hole by Y, then by X.
func anchorHole(holes []DrillTarget) int {
	best := 0
	for i, h := range holes {
		if h.Y < holes[best].Y || (h.Y == holes[best].Y && h.X < holes[best].X) {
			best = i
		}
	}
	return best
}

// nearestHole returns the index of the hole in holes closest to from, breaking equal distances by
// the lower (Y, X) so the tour is deterministic.
func nearestHole(from DrillTarget, holes []DrillTarget) int {
	best, bestD := 0, holeDist2(from, holes[0])
	for i := 1; i < len(holes); i++ {
		d := holeDist2(from, holes[i])
		if d < bestD || (d == bestD && (holes[i].Y < holes[best].Y || (holes[i].Y == holes[best].Y && holes[i].X < holes[best].X))) {
			best, bestD = i, d
		}
	}
	return best
}

// holeDist2 is the squared XY distance between two holes (squared avoids a needless sqrt).
func holeDist2(a, b DrillTarget) float64 {
	dx, dy := a.X-b.X, a.Y-b.Y
	return dx*dx + dy*dy
}

// takeAt removes and returns the hole at index i from *holes (order of the rest is not preserved,
// which is fine — the tour re-picks by distance each step).
func takeAt(holes *[]DrillTarget, i int) DrillTarget {
	s := *holes
	h := s[i]
	s[i] = s[len(s)-1]
	*holes = s[:len(s)-1]
	return h
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
	return orderedHoles(targets), nil
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
