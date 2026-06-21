// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/adaptive"
	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
	"oblikovati.org/cam/bridge/geom2d"
)

// adaptiveSolverTolerance is the step resolution (mm) handed to the constant-engagement solver.
const adaptiveSolverTolerance = 0.1

// adaptiveDefaultStepOver is the radial engagement (fraction of tool diameter) used when the op's
// StepOver is unset — 10%, the HSM signature.
const adaptiveDefaultStepOver = 0.1

// solverToolpath generates the adaptive clearing toolpath with the faithful constant-engagement
// solver: it builds the solver config from the op, runs the region clearing once in XY, and emits
// the resulting helix-entry + cutting/link moves at each depth level. Returns adaptive.ErrUnavailable
// (wrapped) when the clearing engine is not built in, so the caller can fall back to the spiral.
func (op *AdaptiveOp) solverToolpath(tc ToolController, feeds gen.Feeds) (gcode.Path, error) {
	cfg := adaptive.Config{
		ToolDiameter:          tc.Tool.Diameter,
		StepOverFactor:        adaptiveStepOverFactor(op.StepOver),
		Tolerance:             adaptiveSolverTolerance,
		StockToLeave:          op.FinishAllowance,
		ForceInsideOut:        true,
		FinishingProfile:      op.FinishAllowance > 0,
		KeepToolDownDistRatio: 3.0,
		OpType:                adaptive.ClearingInside,
	}
	boundary := polygonToDPath(op.Boundary)
	paths := []adaptive.DPath{boundary}
	for _, isl := range op.Islands {
		paths = append(paths, polygonToDPath(isl))
	}

	outputs, err := adaptive.Execute(cfg, []adaptive.DPath{boundary}, paths, nil)
	if err != nil {
		return gcode.Path{}, err
	}

	var cmds []gcode.Command
	for _, z := range gen.DepthLevels(op.StartDepth, op.FinalDepth, op.StepDown) {
		for _, out := range outputs {
			cmds = append(cmds, adaptiveRegionCommands(out, z, feeds)...)
		}
	}
	if !hasCuttingMove(cmds) {
		return gcode.Path{}, fmt.Errorf("adaptive clearing: tool diameter %g is too large to enter the region (area %g)", tc.Tool.Diameter, op.Boundary.Area())
	}
	return op.frame(cmds), nil
}

// adaptiveStepOverFactor converts the op's step-over (a fraction of the tool diameter) to the
// solver's radial-engagement factor (a fraction of the tool radius), i.e. ×2.
func adaptiveStepOverFactor(stepOver float64) float64 {
	if stepOver <= 0 {
		stepOver = adaptiveDefaultStepOver
	}
	return stepOver * 2
}

// adaptiveRegionCommands frames one cleared region at depth z: rapid in and plunge at the start
// point, walk the region's adaptive/link toolpaths, then retract.
func adaptiveRegionCommands(out adaptive.Output, z float64, feeds gen.Feeds) []gcode.Command {
	if len(out.AdaptivePaths) == 0 {
		return nil
	}
	cmds := []gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"Z": feeds.ClearanceZ}),
		gcode.NewCommand("G0", map[string]float64{"X": out.StartPoint.X, "Y": out.StartPoint.Y}),
		gcode.NewCommand("G0", map[string]float64{"Z": feeds.SafeZ}),
		gcode.NewCommand("G1", map[string]float64{"Z": z, "F": feeds.Vert}),
	}
	for _, tp := range out.AdaptivePaths {
		cmds = append(cmds, adaptiveMotionCommands(tp, z, feeds)...)
	}
	return append(cmds, gcode.NewCommand("G0", map[string]float64{"Z": feeds.ClearanceZ}))
}

// adaptiveMotionCommands emits one sub-path: engaged cuts as G1 feeds, a cleared link as a stay-down
// rapid, and a not-clear link as a retract / reposition / plunge (the always-retract simplification
// where keeping the tool down is unsafe).
func adaptiveMotionCommands(tp adaptive.TPath, z float64, feeds gen.Feeds) []gcode.Command {
	if len(tp.Pts) == 0 {
		return nil
	}
	switch tp.Motion {
	case adaptive.MotionLinkNotClear:
		end := tp.Pts[len(tp.Pts)-1]
		return []gcode.Command{
			gcode.NewCommand("G0", map[string]float64{"Z": feeds.ClearanceZ}),
			gcode.NewCommand("G0", map[string]float64{"X": end.X, "Y": end.Y}),
			gcode.NewCommand("G0", map[string]float64{"Z": feeds.SafeZ}),
			gcode.NewCommand("G1", map[string]float64{"Z": z, "F": feeds.Vert}),
		}
	case adaptive.MotionLinkClear:
		cmds := make([]gcode.Command, 0, len(tp.Pts))
		for _, p := range tp.Pts {
			cmds = append(cmds, gcode.NewCommand("G0", map[string]float64{"X": p.X, "Y": p.Y}))
		}
		return cmds
	default: // MotionCutting
		cmds := make([]gcode.Command, 0, len(tp.Pts))
		for _, p := range tp.Pts {
			cmds = append(cmds, gcode.NewCommand("G1", map[string]float64{"X": p.X, "Y": p.Y, "F": feeds.Horiz}))
		}
		return cmds
	}
}

// hasCuttingMove reports whether any command is an XY feed (so the region produced real cutting).
func hasCuttingMove(cmds []gcode.Command) bool {
	for _, c := range cmds {
		if c.Name == "G1" {
			if _, hasX := c.Params["X"]; hasX {
				return true
			}
		}
	}
	return false
}

// polygonToDPath converts a millimetre polygon to the adaptive solver's point list.
func polygonToDPath(poly geom2d.Polygon) adaptive.DPath {
	out := make(adaptive.DPath, len(poly))
	for i, p := range poly {
		out[i] = adaptive.DoublePoint{X: p.X, Y: p.Y}
	}
	return out
}
