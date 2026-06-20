// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
)

// FeatureFlag enumerates the property groups an operation uses. In FreeCAD the flags drive
// which properties get *added* to an op dynamically; in Go the fields always exist, so the
// flags instead declare an op's capabilities for the UI and for validation (e.g. only an op
// with FeatureStepDown exposes a step-down control). Ports the FeatureTool/FeatureDepths/…
// bit system of FreeCAD's ObjectOp.
type FeatureFlag uint32

const (
	FeatureTool         FeatureFlag = 1 << iota // a tool controller (feeds/speeds)
	FeatureDepths                               // start/final depth
	FeatureHeights                              // clearance/safe/retract heights
	FeatureStepDown                             // multi-pass Z step-down
	FeatureBaseGeometry                         // driven by selected faces/edges (a contour)
	FeatureCoolant                              // coolant control
)

// Has reports whether the flag set includes x.
func (f FeatureFlag) Has(x FeatureFlag) bool { return f&x != 0 }

// Operation is one machining operation that produces a toolpath from a Job's geometry and
// tools. Mirrors the role of FreeCAD's Path/Op/Base.ObjectOp: each concrete op (Drilling,
// Profile, …) resolves its tool controller, reads its driving geometry, and appends moves.
type Operation interface {
	Label() string            // display label, also emitted as a path comment
	Active() bool             // inactive ops are skipped by Job.GenerateAll
	SetActive(bool)           // enable/disable the operation
	ToolControllerIndex() int // index into Job.Tools
	Features() FeatureFlag    // the property groups this op uses
	Clone() Operation         // a deep copy (for duplicating an operation)
	Execute(job *Job) (gcode.Path, error)
}

// OpBase holds the properties common to every operation — the tool controller it runs
// under and the depth/height envelope (the planes a tool rapids to, retracts to, and cuts
// between). Concrete operations embed it for the shared accessors and the standard
// clearance/retract framing (see frame). All heights/depths are millimetres, measured in
// the part's Z. Ports the FeatureHeights/FeatureDepths property group of FreeCAD's
// ObjectOp.
type OpBase struct {
	OpLabel        string
	IsActive       bool
	ToolController int // index into Job.Tools

	ClearanceHeight float64 // safe rapid plane above the part (mm)
	SafeHeight      float64 // feed-in transition plane (mm)
	RetractHeight   float64 // canned-cycle R plane (mm)
	StartDepth      float64 // top of cut (mm)
	FinalDepth      float64 // bottom of cut (mm)
	Coolant         string  // "none" | "flood" | "mist"

	Dressups []Dressup // toolpath post-processes applied after framing (tabs, dogbone, …)
}

// CoolantMode returns the operation's coolant mode, defaulting an unset value to "none".
func (b *OpBase) CoolantMode() string {
	if b.Coolant == "" {
		return CoolantNone
	}
	return b.Coolant
}

// Coolant modes emitted around an operation (flood = M8, mist = M7, off = M9).
const (
	CoolantNone  = "none"
	CoolantFlood = "flood"
	CoolantMist  = "mist"
)

// Label implements Operation.
func (b *OpBase) Label() string { return b.OpLabel }

// Active implements Operation.
func (b *OpBase) Active() bool { return b.IsActive }

// SetActive implements Operation.
func (b *OpBase) SetActive(v bool) { b.IsActive = v }

// AppendDressup adds a toolpath dressup to the operation's chain.
func (b *OpBase) AppendDressup(d Dressup) { b.Dressups = append(b.Dressups, d) }

// ClearDressups removes all dressups from the operation.
func (b *OpBase) ClearDressups() { b.Dressups = nil }

// DressupCount reports how many dressups the operation carries.
func (b *OpBase) DressupCount() int { return len(b.Dressups) }

// dressupHolder is the subset of an operation used to manage its dressup chain (every operation
// satisfies it via OpBase).
type dressupHolder interface {
	AppendDressup(Dressup)
	ClearDressups()
	DressupCount() int
}

// ToolControllerIndex implements Operation.
func (b *OpBase) ToolControllerIndex() int { return b.ToolController }

// resolveTool returns the operation's tool controller, erroring (with the operation label
// and the bad index) when the job has no such controller — the message names the offending
// value per the project's exception rule.
func (b *OpBase) resolveTool(job *Job) (ToolController, error) {
	tc, ok := job.toolController(b.ToolController)
	if !ok {
		return ToolController{}, fmt.Errorf("operation %q references tool controller %d, but the job has %d controller(s)",
			b.OpLabel, b.ToolController, len(job.Tools))
	}
	return tc, nil
}

// frame wraps an operation's cutting commands in the standard envelope: a leading label
// comment and a trailing rapid to the clearance plane, mirroring ObjectOp.execute's
// commandlist initialisation and final `G0 Z=ClearanceHeight`. The op supplies only its
// cutting moves; framing keeps every operation's entry/exit consistent.
func (b *OpBase) frame(cutting []gcode.Command) gcode.Path {
	cmds := make([]gcode.Command, 0, len(cutting)+2)
	cmds = append(cmds, gcode.NewCommand("("+b.OpLabel+")", nil))
	cmds = append(cmds, cutting...)
	cmds = append(cmds, gcode.NewCommand("G0", map[string]float64{"Z": b.ClearanceHeight}))
	return b.applyDressups(gcode.NewPath(cmds))
}

// applyDressups runs the op's dressup chain over a framed path in order. With no dressups it
// returns the path unchanged, so every op gains dressup support at no cost.
func (b *OpBase) applyDressups(path gcode.Path) gcode.Path {
	for _, d := range b.Dressups {
		path = d.Apply(path)
	}
	return path
}
