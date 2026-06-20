// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"errors"
	"fmt"
	"math"

	"oblikovati.org/cam/bridge/gcode"
)

// ThreadMillParams configure single-point thread milling: a thread is cut into a pre-drilled
// hole (internal) or onto a boss (external) by helically interpolating the tool around the
// thread axis, advancing one Pitch per revolution. The tool centre orbits at the major radius
// offset by the tool radius — inward for an internal thread, outward for an external one.
type ThreadMillParams struct {
	MajorRadius   float64 // mm — nominal thread radius (crest of an internal thread)
	ToolRadius    float64 // mm — thread-mill cutter radius
	Pitch         float64 // mm — thread lead (axial advance per revolution)
	Internal      bool    // internal thread (in a hole) vs external (on a boss)
	Climb         bool    // climb vs conventional milling
	RetractHeight float64 // mm — height to retract to after the thread; raised to the top if below
}

// GenerateThreadMill produces the thread-milling toolpath between the thread's top and bottom
// centre points (a vertical edge): rapid over the axis, drop to the thread top, arc out to the
// thread radius, helically interpolate down the thread one pitch per turn, arc back to the axis,
// and retract. Emits G2/G3 arcs with I/J centre offsets — two 180° arcs per turn plus a half-
// circle lead-in/out, so the path stays clear of the wall on entry and exit.
func GenerateThreadMill(top, bottom gcode.Vector3, p ThreadMillParams) ([]gcode.Command, error) {
	if err := validateThreadMill(top, bottom, p); err != nil {
		return nil, err
	}
	orbit := p.MajorRadius - p.ToolRadius
	if !p.Internal {
		orbit = p.MajorRadius + p.ToolRadius
	}
	if orbit <= 0 {
		return nil, fmt.Errorf("thread mill tool radius %g too large for major radius %g", p.ToolRadius, p.MajorRadius)
	}
	t := threadState{cx: top.X, cy: top.Y, orbit: orbit, arc: threadArc(p), retractZ: math.Max(p.RetractHeight, top.Z)}
	turns := int(math.Ceil(round6((top.Z - bottom.Z) / p.Pitch)))
	if turns < 1 {
		turns = 1
	}
	return t.toolpath(top.Z, bottom.Z, turns), nil
}

// threadState carries the resolved thread geometry through the per-move generation.
type threadState struct {
	cx, cy   float64
	orbit    float64
	arc      string
	retractZ float64
}

// toolpath assembles the full thread cut: position, lead-in, the descending helix, lead-out,
// and retract.
func (t threadState) toolpath(topZ, bottomZ float64, turns int) []gcode.Command {
	zs := linspace(topZ, bottomZ, 2*turns+1)
	cmds := []gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"X": t.cx, "Y": t.cy}),
		gcode.NewCommand("G0", map[string]float64{"Z": topZ}),
		t.leadArc(t.cx, t.cx+t.orbit, topZ), // half-circle from the axis out to the thread radius
	}
	for i := 1; i <= turns; i++ {
		cmds = append(cmds, t.halfArc(-t.orbit, zs[2*i-1]), t.halfArc(t.orbit, zs[2*i]))
	}
	return append(cmds,
		t.leadArc(t.cx+t.orbit, t.cx, bottomZ), // half-circle from the thread radius back to the axis
		gcode.NewCommand("G0", map[string]float64{"Z": t.retractZ}),
	)
}

// halfArc is one 180° thread arc ending at the axis offset dx (Y on the axis) at depth z, with
// the centre on the thread axis. Mirrors the helix generator's half-turn.
func (t threadState) halfArc(dx, z float64) gcode.Command {
	return gcode.NewCommand(t.arc, map[string]float64{"X": t.cx + dx, "Y": t.cy, "Z": z, "I": dx, "J": 0})
}

// leadArc is a half-circle (on the Y=cy diameter) from absolute X fromX to toX at depth z, used
// to ease on/off the thread; its centre sits midway between the two points.
func (t threadState) leadArc(fromX, toX, z float64) gcode.Command {
	mid := (fromX + toX) / 2
	return gcode.NewCommand(t.arc, map[string]float64{"X": toX, "Y": t.cy, "Z": z, "I": mid - fromX, "J": 0})
}

// threadArc picks the arc direction (G2 cw / G3 ccw) from the cut direction and thread side:
// climb reverses an internal thread; an external thread reverses again.
func threadArc(p ThreadMillParams) string {
	ccw := p.Climb
	if !p.Internal {
		ccw = !ccw
	}
	if ccw {
		return "G3"
	}
	return "G2"
}

// validateThreadMill rejects illegal geometry and parameters.
func validateThreadMill(top, bottom gcode.Vector3, p ThreadMillParams) error {
	if !isClose(top.X-bottom.X, 0) || !isClose(top.Y-bottom.Y, 0) {
		return fmt.Errorf("thread axis is not aligned with Z: top=%v bottom=%v", top, bottom)
	}
	if top.Z <= bottom.Z {
		return errors.New("thread top must be above its bottom")
	}
	if p.MajorRadius <= 0 {
		return fmt.Errorf("thread major radius must be > 0, got %g", p.MajorRadius)
	}
	if p.Pitch <= 0 {
		return fmt.Errorf("thread pitch must be > 0, got %g", p.Pitch)
	}
	return nil
}
