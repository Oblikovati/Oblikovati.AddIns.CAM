// SPDX-License-Identifier: GPL-2.0-only

package dressup

import (
	"math"
	"strconv"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

const pi = math.Pi

// mnvr builds the move list for a G-code maneuver string ("G1X1/G1Y1"), starting at the
// origin.
func mnvr(s string) []move {
	path := parseManeuver(s)
	var moves []move
	var p gcode.Vector3
	for _, c := range path {
		nx, ny, _, hasXY := endpoint(c, p.X, p.Y, 0)
		if hasXY {
			end := gcode.Vector3{X: nx, Y: ny}
			moves = append(moves, moveOf(c, p.X, p.Y, nx, ny))
			p = end
		}
	}
	return moves
}

// parseManeuver tokenizes a compact maneuver string into commands. Commands run together
// ("G2Y2J1G1X0" is two) and are separated by an optional "/"; a new command begins at each
// G/M code, the way a G-code parser feeds such strings.
func parseManeuver(s string) []gcode.Command {
	var cmds []gcode.Command
	i := 0
	for i < len(s) {
		if s[i] != 'G' && s[i] != 'M' {
			i++ // skip separators / stray characters
			continue
		}
		name := string(s[i])
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			name += string(s[i])
			i++
		}
		params := map[string]float64{}
		for i < len(s) && s[i] != 'G' && s[i] != 'M' && s[i] != '/' {
			addr := string(s[i])
			i++
			j := i
			for j < len(s) && isNumByte(s[j]) {
				j++
			}
			v, _ := strconv.ParseFloat(s[i:j], 64)
			params[addr] = v
			i = j
		}
		cmds = append(cmds, gcode.NewCommand(name, params))
	}
	return cmds
}

// isNumByte reports whether b can appear in a signed decimal number.
func isNumByte(b byte) bool {
	return b == '+' || b == '-' || b == '.' || (b >= '0' && b <= '9')
}

// kinkOf builds the single kink of a two-move maneuver string.
func kinkOf(s string) kink {
	moves := mnvr(s)
	return newKink(moves[0], moves[1])
}

// createKinks forms a kink at each junction of a maneuver, closing the loop when it returns
// to its start.
func createKinks(moves []move) []kink {
	var kinks []kink
	for j := 1; j < len(moves); j++ {
		kinks = append(kinks, newKink(moves[j-1], moves[j]))
	}
	if n := len(moves); n >= 2 && roughlySame(moves[0].begin, moves[n-1].end) {
		kinks = append(kinks, newKink(moves[n-1], moves[0]))
	}
	return kinks
}

// TestKinkDeflections checks the signed turn at each corner.
func TestKinkDeflections(t *testing.T) {
	cases := []struct {
		maneuver string
		want     []float64
	}{
		{"G1X1/G1Y1", []float64{1.57}},
		{"G1X1/G1Y-1", []float64{-1.57}},
		{"G1X1/G1Y1/G1X0", []float64{1.57, 1.57}},
		{"G1X1/G1Y1/G1X0/G1Y0", []float64{1.57, 1.57, 1.57, 1.57}},
		{"G1Y1/G1X1", []float64{-1.57}},
		{"G1Y1/G1X1/G1Y0/G1X0", []float64{-1.57, -1.57, -1.57, -1.57}},
		{"G1X1/G3Y2J1", []float64{0.00}},  // tangential arc — no kink
		{"G1X1/G2Y2J1", []float64{-3.14}}, // folding-back arc
		{"G1X1/G2Y2J1G1X0", []float64{-3.14, 3.14}},
	}
	for _, tc := range cases {
		kinks := createKinks(mnvr(tc.maneuver))
		if len(kinks) != len(tc.want) {
			t.Fatalf("%s: got %d kinks, want %d", tc.maneuver, len(kinks), len(tc.want))
		}
		for i, k := range kinks {
			if math.Abs(k.defl-tc.want[i]) > 0.01 {
				t.Errorf("%s kink %d: deflection %.2f, want %.2f", tc.maneuver, i, k.defl, tc.want[i])
			}
		}
	}
}

// TestKinkDetection ports test30: which corners qualify at a deflection threshold.
func TestKinkDetection(t *testing.T) {
	count := func(maneuver string, side string) int {
		p := DogboneParams{Length: 1, MinAngle: pi / 4, Side: side}
		n := 0
		for _, k := range createKinks(mnvr(maneuver)) {
			if qualifies(k, p) {
				n++
			}
		}
		return n
	}
	if got := count("G1X1/G1Y1/G1X0/G1Y0", SideLeft); got != 4 {
		t.Errorf("square loop left bones = %d, want 4", got)
	}
	if got := count("G1X1/G1Y1/G1X0/G1Y0", SideRight); got != 0 {
		t.Errorf("square loop right bones = %d, want 0", got)
	}
	// flat (collinear) corner produces no bone
	if got := count("G1X1/G1X3Y1/G1X0/G1Y0", SideLeft); got != 3 {
		t.Errorf("loop with a flat corner left bones = %d, want 3", got)
	}
	// perpendicular arc yields a single right-side bone
	if got := count("G1X1/G3X3I1/G1Y1/G1X0/G1Y0", SideRight); got != 1 {
		t.Errorf("perpendicular-arc right bones = %d, want 1", got)
	}
}

// TestNormAngles ports test70: the corner bisector angle (degrees).
func TestNormAngles(t *testing.T) {
	cases := []struct {
		maneuver string
		wantDeg  float64
	}{
		{"G1X1/G1Y+1", -45}, {"G1X1/G1Y-1", 45},
		{"G1X1/G1X2Y1", -67.5}, {"G1X1/G1X2Y-1", 67.5},
		{"G1Y1/G1X+1", 135}, {"G1Y1/G1X-1", 45},
		{"G1X-1/G1Y+1", -135}, {"G1X-1/G1Y-1", 135},
		{"G1Y-1/G1X-1", -45}, {"G1Y-1/G1X+1", -135},
	}
	for _, tc := range cases {
		got := 180 * kinkOf(tc.maneuver).normAngle() / pi
		if math.Abs(got-tc.wantDeg) > 0.01 {
			t.Errorf("%s normAngle = %.2f°, want %.2f°", tc.maneuver, got, tc.wantDeg)
		}
	}
}

// boneXY returns the changed-axis values of a bone's two moves, defaulting an unset axis to
// the corner so a single-axis (tangent) bone reports a full point.
func boneXY(in, out gcode.Command, corner gcode.Vector3) (ix, iy, ox, oy float64) {
	get := func(c gcode.Command, addr string, def float64) float64 {
		if v, ok := c.Params[addr]; ok {
			return v
		}
		return def
	}
	return get(in, "X", corner.X), get(in, "Y", corner.Y), get(out, "X", corner.X), get(out, "Y", corner.Y)
}

func assertBone(t *testing.T, label string, in, out gcode.Command, corner gcode.Vector3, wx, wy, wox, woy float64) {
	t.Helper()
	ix, iy, ox, oy := boneXY(in, out, corner)
	const eps = 0.02
	if math.Abs(ix-wx) > eps || math.Abs(iy-wy) > eps || math.Abs(ox-wox) > eps || math.Abs(oy-woy) > eps {
		t.Errorf("%s: bone in (%.3f,%.3f) out (%.3f,%.3f); want in (%.2f,%.2f) out (%.2f,%.2f)",
			label, ix, iy, ox, oy, wx, wy, wox, woy)
	}
}

// TestDogboneBones ports test71: bones along the corner bisector.
func TestDogboneBones(t *testing.T) {
	gen := func(maneuver string) (gcode.Command, gcode.Command, gcode.Vector3) {
		k := kinkOf(maneuver)
		in, out := generateBone(k, 1, boneAngle(StyleDogbone, k))
		return in, out, k.position()
	}
	in, out, c := gen("G1X1/G1Y1")
	assertBone(t, "dogbone X1Y1", in, out, c, 1.7071, -0.7071, 1.0, 0.0)
	in, out, c = gen("G1X1/G1X3Y-1")
	assertBone(t, "dogbone X1X3Y-1", in, out, c, 1.230, 0.973, 1.0, 0.0)
	in, out, c = gen("G1X1Y1/G1X2")
	assertBone(t, "dogbone X1Y1X2", in, out, c, 0.617, 1.924, 1.0, 1.0)
}

// TestTBoneHorizontalBones ports test40: horizontal T-bones around a CCW square loop.
func TestTBoneHorizontalBones(t *testing.T) {
	moves := mnvr("G1X1/G1Y1/G1X0/G1Y0")
	kinks := createKinks(moves)
	want := [][4]float64{{2, 0, 1, 0}, {2, 1, 1, 1}, {-1, 1, 0, 1}, {-1, 0, 0, 0}}
	for i, k := range kinks {
		in, out := generateBone(k, 1, boneAngle(StyleTBoneH, k))
		assertBone(t, "tbone-h", in, out, k.position(), want[i][0], want[i][1], want[i][2], want[i][3])
	}
}

// TestTBoneVerticalBones ports test50: vertical T-bones around a CCW square loop.
func TestTBoneVerticalBones(t *testing.T) {
	moves := mnvr("G1X1/G1Y1/G1X0/G1Y0")
	kinks := createKinks(moves)
	want := [][4]float64{{1, -1, 1, 0}, {1, 2, 1, 1}, {0, 2, 0, 1}, {0, -1, 0, 0}}
	for i, k := range kinks {
		in, out := generateBone(k, 1, boneAngle(StyleTBoneV, k))
		assertBone(t, "tbone-v", in, out, k.position(), want[i][0], want[i][1], want[i][2], want[i][3])
	}
}

// TestTBoneOnShortLong ports parts of test60: bones aligned to the shorter / longer edge.
func TestTBoneOnShortLong(t *testing.T) {
	k := kinkOf("G1X1/G1Y2") // short horizontal edge (len 1) vs long vertical (len 2)
	in, out := generateBone(k, 1, boneAngle(StyleTBoneShrt, k))
	assertBone(t, "on-short horizontal", in, out, k.position(), 1, -1, 1, 0)

	k = kinkOf("G1X2/G1Y1") // long horizontal edge (len 2) vs short vertical (len 1)
	in, out = generateBone(k, 1, boneAngle(StyleTBoneLong, k))
	assertBone(t, "on-long horizontal", in, out, k.position(), 2, -1, 2, 0)
}
