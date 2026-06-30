// SPDX-License-Identifier: GPL-2.0-only

package gcode

import "testing"

// plunges returns every feed move (G1) that lowers Z, as (x,y,z) of its endpoint — the carving
// moves a canned cycle must expand into.
func plunges(p Path) []Vector3 {
	var out []Vector3
	var cur Vector3
	for _, c := range p.Commands {
		if x, ok := c.Params["X"]; ok {
			cur.X = x
		}
		if y, ok := c.Params["Y"]; ok {
			cur.Y = y
		}
		if z, ok := c.Params["Z"]; ok {
			cur.Z = z
		}
		if c.Name == "G1" {
			out = append(out, cur)
		}
	}
	return out
}

// hasName reports whether any command in the path carries the given G/M name.
func hasName(p Path, name string) bool {
	for _, c := range p.Commands {
		if c.Name == name {
			return true
		}
	}
	return false
}

// TestExpandG81PlungesAndRetractsToR checks a G99 drill cycle becomes position → rapid-to-R →
// feed-plunge → rapid-out-to-R, and the opaque cycle word is gone.
func TestExpandG81PlungesAndRetractsToR(t *testing.T) {
	in := NewPath([]Command{
		NewCommand("G99", nil),
		NewCommand("G81", map[string]float64{"X": 5, "Y": 6, "Z": -10, "R": 2}),
	})
	out := ExpandCannedCycles(in)
	if hasName(out, "G81") {
		t.Error("G81 cycle was not expanded")
	}
	pl := plunges(out)
	if len(pl) != 1 || pl[0] != (Vector3{X: 5, Y: 6, Z: -10}) {
		t.Fatalf("plunge = %+v, want one to (5,6,-10)", pl)
	}
	// last move is the rapid retract to the R plane (G99)
	last := out.Commands[len(out.Commands)-1]
	if last.Name != "G0" || last.Params["Z"] != 2 {
		t.Errorf("retract = %+v, want G0 Z2", last)
	}
}

// TestExpandG98RetractsToInitialZ checks G98 retracts to the Z held before the cycle (the clearance
// plane), not the R plane.
func TestExpandG98RetractsToInitialZ(t *testing.T) {
	in := NewPath([]Command{
		NewCommand("G0", map[string]float64{"Z": 15}), // clearance — the initial plane
		NewCommand("G98", nil),
		NewCommand("G81", map[string]float64{"X": 0, "Y": 0, "Z": -5, "R": 1}),
	})
	out := ExpandCannedCycles(in)
	last := out.Commands[len(out.Commands)-1]
	if last.Name != "G0" || last.Params["Z"] != 15 {
		t.Errorf("G98 retract = %+v, want G0 Z15 (initial plane)", last)
	}
}

// TestExpandModalRepeat checks a bare X/Y point under an active cycle repeats the cycle at the new
// position with the modal depth and R plane.
func TestExpandModalRepeat(t *testing.T) {
	in := NewPath([]Command{
		NewCommand("G99", nil),
		NewCommand("G81", map[string]float64{"X": 0, "Y": 0, "Z": -3, "R": 1}),
		NewCommand("", map[string]float64{"X": 10, "Y": 0}), // modal repeat
	})
	pl := plunges(ExpandCannedCycles(in))
	if len(pl) != 2 || pl[1] != (Vector3{X: 10, Y: 0, Z: -3}) {
		t.Fatalf("plunges = %+v, want two, second at (10,0,-3)", pl)
	}
}

// TestG80CancelsCycle checks G80 ends the modal cycle so a following point is not drilled.
func TestG80CancelsCycle(t *testing.T) {
	in := NewPath([]Command{
		NewCommand("G99", nil),
		NewCommand("G81", map[string]float64{"X": 0, "Y": 0, "Z": -3, "R": 1}),
		NewCommand("G80", nil),
		NewCommand("", map[string]float64{"X": 10, "Y": 0}),
	})
	if pl := plunges(ExpandCannedCycles(in)); len(pl) != 1 {
		t.Errorf("plunges = %d, want 1 (cycle cancelled before the second point)", len(pl))
	}
}

// rapidZ returns the Z of every rapid (G0) move that sets Z, in order.
func rapidZ(p Path) []float64 {
	var out []float64
	for _, c := range p.Commands {
		if z, ok := c.Params["Z"]; ok && c.Name == "G0" {
			out = append(out, z)
		}
	}
	return out
}

func count(vals []float64, want float64) int {
	n := 0
	for _, v := range vals {
		if v == want {
			n++
		}
	}
	return n
}

// TestExpandG83Pecks checks deep-hole drilling descends in Q increments and fully retracts to the R
// plane between pecks (the chip-clearing woodpecker), reaching the hole bottom.
func TestExpandG83Pecks(t *testing.T) {
	in := NewPath([]Command{
		NewCommand("G99", nil),
		NewCommand("G83", map[string]float64{"X": 0, "Y": 0, "Z": -10, "R": 2, "Q": 3}),
	})
	out := ExpandCannedCycles(in)
	gotZ := []float64{}
	for _, p := range plunges(out) {
		gotZ = append(gotZ, p.Z)
	}
	want := []float64{-1, -4, -7, -10} // R=2 stepping down by 3, last lands on the bottom
	if len(gotZ) != len(want) {
		t.Fatalf("peck feeds = %v, want %v", gotZ, want)
	}
	for i := range want {
		if gotZ[i] != want[i] {
			t.Errorf("peck %d feed Z = %v, want %v", i, gotZ[i], want[i])
		}
	}
	if c := count(rapidZ(out), 2); c < 4 { // approach + 3 chip-clear retracts to R
		t.Errorf("full retracts to R = %d, want >= 4 (G83 retracts each peck)", c)
	}
}

// TestExpandG73ChipBreak checks high-speed peck drilling descends in Q increments but only backs off
// a small amount between pecks (no full retract to R).
func TestExpandG73ChipBreak(t *testing.T) {
	in := NewPath([]Command{
		NewCommand("G99", nil),
		NewCommand("G73", map[string]float64{"X": 0, "Y": 0, "Z": -10, "R": 2, "Q": 3}),
	})
	out := ExpandCannedCycles(in)
	if n := len(plunges(out)); n != 4 {
		t.Fatalf("peck feeds = %d, want 4", n)
	}
	if c := count(rapidZ(out), 2); c > 2 { // only the approach and the final retract reach R
		t.Errorf("retracts to R = %d, want <= 2 (G73 stays in the hole)", c)
	}
	backoff := false
	for _, z := range rapidZ(out) {
		if z > -10 && z < 2 && z != 2 { // a partial back-off strictly inside the hole
			backoff = true
		}
	}
	if !backoff {
		t.Error("no chip-break back-off found between pecks")
	}
}

// TestExpandPassesNonCyclesThrough checks a program without cycles is returned unchanged.
func TestExpandPassesNonCyclesThrough(t *testing.T) {
	in := NewPath([]Command{
		NewCommand("G0", map[string]float64{"X": 1, "Y": 2, "Z": 5}),
		NewCommand("G1", map[string]float64{"Z": 0}),
		NewCommand("G1", map[string]float64{"X": 10}),
	})
	out := ExpandCannedCycles(in)
	if len(out.Commands) != len(in.Commands) {
		t.Errorf("length changed %d → %d for a cycle-free program", len(in.Commands), len(out.Commands))
	}
}
