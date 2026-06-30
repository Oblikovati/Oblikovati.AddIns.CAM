// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
	"oblikovati.org/cam/bridge/geom2d"
)

// FeatureFlag enumerates the property groups an operation uses: the fields always exist on the
// Go structs, so the flags declare an op's capabilities for the UI and for validation (e.g. only
// an op with FeatureStepDown exposes a step-down control).
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
// tools. Each concrete op (Drilling,
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
// the part's Z (the FeatureHeights/FeatureDepths property group).
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
	PauseAfter      bool    // emit an optional stop (M1) after this operation, e.g. to inspect
	FeedScale       float64 // multiplier on the tool's cutting/plunge feed for this op (e.g. 0.5 for a finishing pass); 0 → full feed

	Dressups []Dressup // toolpath post-processes applied after framing (tabs, dogbone, …)

	// roomAt, when set by an op that knows its driving region, reports the distance from a point to
	// the nearest wall so room-aware dressups (e.g. the helical ramp) can keep their geometry inside
	// the boundary. It is runtime-only state, never persisted. See setRoom.
	roomAt func(x, y float64) float64
}

// roomAwareDressup is the optional capability a dressup implements when it can use a wall-clearance
// function — the helical ramp shrinks its entry circle to fit. applyDressups offers roomAt only to
// dressups that implement it and only when the op has supplied one.
type roomAwareDressup interface {
	ApplyWithRoom(gcode.Path, func(x, y float64) float64) gcode.Path
}

// setRoom records the op's wall-clearance function, to be offered to room-aware dressups. An op
// calls it from Execute once it has its boundary, before framing.
func (b *OpBase) setRoom(roomAt func(x, y float64) float64) { b.roomAt = roomAt }

// setBoundaryRoom is the common case of setRoom: the wall clearance is the distance to a closed
// region boundary, so a clearing op (pocket, adaptive) can keep helical entries inside it.
func (b *OpBase) setBoundaryRoom(boundary geom2d.Polygon) {
	b.setRoom(func(x, y float64) float64 {
		return geom2d.DistanceToBoundary(geom2d.Point2{X: x, Y: y}, boundary)
	})
}

// feedFactor returns the operation's feed multiplier, defaulting an unset (zero) FeedScale to 1.
func (b *OpBase) feedFactor() float64 {
	if b.FeedScale > 0 {
		return b.FeedScale
	}
	return 1
}

// feeds packs the operation's clearance/safe heights and the controller's cutting/plunge feeds
// (scaled by FeedScale) into the generator's Feeds — the shared feed set every milling op walks at.
func (b *OpBase) feeds(tc ToolController) gen.Feeds {
	f := b.feedFactor()
	return gen.Feeds{Vert: tc.VertFeed * f, Horiz: tc.HorizFeed * f, ClearanceZ: b.ClearanceHeight, SafeZ: b.SafeHeight}
}

// CoolantMode returns the operation's coolant mode, defaulting an unset value to "none".
func (b *OpBase) CoolantMode() string {
	if b.Coolant == "" {
		return CoolantNone
	}
	return b.Coolant
}

// PausesAfter reports whether an optional stop should follow this operation.
func (b *OpBase) PausesAfter() bool { return b.PauseAfter }

// RetractHeights returns the operation's safe (feed-in) and clearance (rapid) planes in millimetres
// — the candidate heights between-cut link planning lifts to when it cannot keep the tool down.
func (b *OpBase) RetractHeights() (safeZ, clearanceZ float64) { return b.SafeHeight, b.ClearanceHeight }

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

// DressupList returns the operation's dressup chain (for editing in the operation editor).
func (b *OpBase) DressupList() []Dressup { return b.Dressups }

// SetDressupParam applies one parameter edit to dressup idx, replacing it with the edited copy.
// Returns whether the edit landed (a known index and an editable dressup).
func (b *OpBase) SetDressupParam(idx int, id, value string) bool {
	if idx < 0 || idx >= len(b.Dressups) {
		return false
	}
	ed, ok := b.Dressups[idx].(DressupEditable)
	if !ok {
		return false
	}
	b.Dressups[idx] = ed.WithParameter(id, value)
	return true
}

// dressupHolder is the subset of an operation used to manage its dressup chain (every operation
// satisfies it via OpBase).
type dressupHolder interface {
	AppendDressup(Dressup)
	ClearDressups()
	DressupCount() int
	DressupList() []Dressup
	SetDressupParam(idx int, id, value string) bool
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
// comment and a trailing rapid to the clearance plane (a final `G0 Z=ClearanceHeight`). The
// op supplies only its
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
		if aware, ok := d.(roomAwareDressup); ok && b.roomAt != nil {
			path = aware.ApplyWithRoom(path, b.roomAt)
			continue
		}
		path = d.Apply(path)
	}
	return path
}
