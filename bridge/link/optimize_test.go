// SPDX-License-Identifier: GPL-2.0-only

package link

import (
	"errors"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// retractPath models a two-cut pocket pass: an approach, cut 1, a full retract to the clearance
// plane (Z=15) with a traverse and a feed-plunge back down, cut 2, and a final retract. It mirrors
// the lift→traverse→safe→feed-plunge sequence the generators emit (see gen/pocket plungeAt).
func retractPath() gcode.Path {
	g0 := func(p map[string]float64) gcode.Command { return gcode.NewCommand("G0", p) }
	g1 := func(p map[string]float64) gcode.Command { return gcode.NewCommand("G1", p) }
	return gcode.NewPath([]gcode.Command{
		g0(map[string]float64{"Z": 15}), g0(map[string]float64{"X": 0, "Y": 0}), g0(map[string]float64{"Z": 2}), // approach (leading)
		g1(map[string]float64{"Z": 0, "F": 100}), g1(map[string]float64{"X": 10, "Y": 0, "F": 200}), // cut 1
		g0(map[string]float64{"Z": 15}), g0(map[string]float64{"X": 20, "Y": 0}), g0(map[string]float64{"Z": 2}), // retract+traverse
		g1(map[string]float64{"Z": 0, "F": 100}), g1(map[string]float64{"X": 30, "Y": 0, "F": 200}), // cut 2
		g0(map[string]float64{"Z": 15}), // final retract (trailing)
	})
}

func countZ(p gcode.Path, z float64) int {
	n := 0
	for _, c := range p.Commands {
		if v, ok := c.Params["Z"]; ok && v == z {
			n++
		}
	}
	return n
}

func TestOptimizeRetractsLowersLiftWhenClear(t *testing.T) {
	// Part top at z=0, so a traverse at the safe plane (2) clears it: the between-cut retract no
	// longer bounces up to the clearance plane (15). The leading approach and trailing final retract
	// (both at 15) are left untouched, so exactly one of the three Z15 lifts is removed.
	probe := fakeProbe{clearance: clearAbove(0)}
	out, err := OptimizeRetracts(retractPath(), 2, 15, probe, 0, 1)
	if err != nil {
		t.Fatalf("OptimizeRetracts: %v", err)
	}
	if got := countZ(out, 15); got != 2 {
		t.Errorf("Z15 lifts after optimize = %d, want 2 (leading approach + trailing retract kept, middle lowered)", got)
	}
}

func TestOptimizeRetractsKeepsFullLiftOverTallIsland(t *testing.T) {
	// An island up to z=10 blocks the safe-plane traverse (2), so the link must keep the full lift to
	// the clearance plane (15) — the always-safe behaviour is preserved.
	probe := fakeProbe{clearance: clearAbove(10)}
	out, err := OptimizeRetracts(retractPath(), 2, 15, probe, 0, 1)
	if err != nil {
		t.Fatalf("OptimizeRetracts: %v", err)
	}
	if got := countZ(out, 15); got != 3 {
		t.Errorf("Z15 lifts after optimize = %d, want 3 (full retract kept over the island)", got)
	}
}

func TestOptimizeRetractsKeepsOriginalWhenNoClearLink(t *testing.T) {
	probe := fakeProbe{clearance: func([]gcode.Vector3, float64) float64 { return 0 }}
	out, err := OptimizeRetracts(retractPath(), 2, 15, probe, 0, 1)
	if err != nil {
		t.Fatalf("OptimizeRetracts: %v", err)
	}
	if got := len(out.Commands); got != len(retractPath().Commands) {
		t.Errorf("no-clear-link path length = %d, want the original %d (untouched)", got, len(retractPath().Commands))
	}
}

func TestOptimizeRetractsLeavesDrillCyclesAlone(t *testing.T) {
	// A canned drill cycle (G81) handles its own retraction and is not a cut here, so the rapids
	// around it are leading/trailing (not between-cut) and must pass through unchanged.
	path := gcode.NewPath([]gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"Z": 15}),
		gcode.NewCommand("G81", map[string]float64{"X": 0, "Y": 0, "Z": -5, "R": 2}),
		gcode.NewCommand("G0", map[string]float64{"Z": 15}),
	})
	probe := fakeProbe{clearance: func([]gcode.Vector3, float64) float64 { return 100 }}
	out, err := OptimizeRetracts(path, 2, 15, probe, 0, 1)
	if err != nil {
		t.Fatalf("OptimizeRetracts: %v", err)
	}
	if len(out.Commands) != 3 || out.Commands[1].Name != "G81" {
		t.Errorf("drill path was altered: %v", out.Commands)
	}
}

func TestOptimizeRetractsPropagatesProbeError(t *testing.T) {
	probe := fakeProbe{err: errors.New("host down")}
	if _, err := OptimizeRetracts(retractPath(), 2, 15, probe, 0, 1); err == nil {
		t.Error("a probe error must surface from the optimizer")
	}
}
