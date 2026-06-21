// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "oblikovati.org/cam/bridge/gcode"

// Job is the top-level machining container: the part(s) it cuts, the stock it cuts from,
// the tool controllers it has loaded, and the ordered operations that produce toolpaths.
// A pared-down job container — enough for the milestone-1 drilling
// slice. The post processor is selected by name and applied to the concatenated
// operation paths (see post.PostList).
type Job struct {
	ModelBodies       []int            // host body indices this job machines
	Stock             Stock            // raw material (mm)
	Tools             []ToolController // loaded tool controllers
	Operations        []Operation      // ordered operations
	GeometryTolerance float64          // chordal tolerance for geometry queries (mm)
	PostProcessor     string           // "linuxcnc" | "grbl"
}

// NewJob creates an empty job with sane milestone-1 defaults.
func NewJob() *Job {
	return &Job{GeometryTolerance: 0.01, PostProcessor: "linuxcnc"}
}

// toolController returns the controller at the given index, or false when out of range so
// an operation can report a missing tool rather than panic.
func (j *Job) toolController(index int) (ToolController, bool) {
	if index < 0 || index >= len(j.Tools) {
		return ToolController{}, false
	}
	return j.Tools[index], true
}

// GenerateAll executes every active operation in order and returns their toolpaths paired
// with the controller each ran under, ready for the post processor. The first error stops
// the job (a bad operation must not silently drop from the program).
func (j *Job) GenerateAll() ([]OperationResult, error) {
	var results []OperationResult
	for _, op := range j.Operations {
		if !op.Active() {
			continue
		}
		path, err := op.Execute(j)
		if err != nil {
			return nil, err
		}
		tc, _ := j.toolController(op.ToolControllerIndex())
		safeZ, clearanceZ := retractHeightsOf(op)
		results = append(results, OperationResult{
			Label:      op.Label(),
			Path:       path,
			Controller: tc,
			Coolant:    coolantOf(op),
			PauseAfter: pauseAfterOf(op),
			SafeZ:      safeZ,
			ClearanceZ: clearanceZ,
		})
	}
	return results, nil
}

// coolantOf reads an operation's coolant mode (operations all carry one via OpBase).
func coolantOf(op Operation) string {
	if c, ok := op.(interface{ CoolantMode() string }); ok {
		return c.CoolantMode()
	}
	return CoolantNone
}

// pauseAfterOf reads whether an operation requests an optional stop after it.
func pauseAfterOf(op Operation) bool {
	if p, ok := op.(interface{ PausesAfter() bool }); ok {
		return p.PausesAfter()
	}
	return false
}

// retractHeightsOf reads an operation's safe and clearance planes (mm), which between-cut link
// planning uses as the candidate retract heights. Operations carry both via OpBase.
func retractHeightsOf(op Operation) (safeZ, clearanceZ float64) {
	if r, ok := op.(interface {
		RetractHeights() (float64, float64)
	}); ok {
		return r.RetractHeights()
	}
	return 0, 0
}

// OperationResult is one operation's generated toolpath plus the controller it ran under
// (the post processor needs the tool number + spindle to emit tool-change and start
// blocks between operations).
type OperationResult struct {
	Label      string
	Path       gcode.Path
	Controller ToolController
	Coolant    string  // "none" | "flood" | "mist"
	PauseAfter bool    // emit an optional stop (M1) after this operation
	SafeZ      float64 // the op's safe (feed-in) plane, mm — for between-cut link planning
	ClearanceZ float64 // the op's clearance (rapid) plane, mm — for between-cut link planning
}
