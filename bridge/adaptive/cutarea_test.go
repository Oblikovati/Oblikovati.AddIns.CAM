// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import (
	"math"
	"testing"

	"oblikovati.org/cam/bridge/clipper"
)

// crescentArea is the closed-form area inside one circle but outside an equal circle whose centre
// is distance d away (radius r): the full disc minus the two-circle lens. This is exactly what
// calcCutArea must compute when no area has been cleared yet.
func crescentArea(r, d float64) float64 {
	lens := 2*r*r*math.Acos(d/(2*r)) - (d/2)*math.Sqrt(4*r*r-d*d)
	return math.Pi*r*r - lens
}

func TestCalcCutAreaDegenerate(t *testing.T) {
	a, c := calcCutArea(clipper.IntPoint{X: 0, Y: 0}, clipper.IntPoint{X: 0, Y: 0}, 1000, nil)
	if a != 0 || c != 0 {
		t.Fatalf("a zero-length move should cut nothing, got area %g conv %g", a, c)
	}
}

func TestCalcCutAreaMatchesCrescent(t *testing.T) {
	// A horizontal move keeps the rotated tool centres exact, so the freshly-cut area with no
	// cleared region is the analytic crescent (disc minus the c1∩c2 lens). conventionalArea is
	// the left half (xtest < c2.X), which is exactly half by symmetry.
	const r, d = 1000.0, 200.0
	area, conv := calcCutArea(clipper.IntPoint{X: 0, Y: 0}, clipper.IntPoint{X: int64(d), Y: 0}, int64(r), nil)
	want := crescentArea(r, d)
	if math.Abs(area-want) > 2.0 {
		t.Fatalf("crescent area = %g, want %g (±2)", area, want)
	}
	if math.Abs(conv-want/2) > 2.0 {
		t.Fatalf("conventional area = %g, want %g (half)", conv, want/2)
	}
}

func TestCalcCutAreaReferenceSlot(t *testing.T) {
	// The solver's reference cut area is the crescent for a half-radius step (d = toolRadius/2).
	const r = 1000.0
	area, _ := calcCutArea(clipper.IntPoint{X: 0, Y: 0}, clipper.IntPoint{X: int64(r / 2), Y: 0}, int64(r), nil)
	if want := crescentArea(r, r/2); math.Abs(area-want) > 2.0 {
		t.Fatalf("reference slot area = %g, want %g", area, want)
	}
}

func TestCalcCutAreaFullyCleared(t *testing.T) {
	// A cleared region that already covers the whole tool neighbourhood: nothing new is cut.
	cleared := clipper.Paths{{{X: -3000, Y: -3000}, {X: 3000, Y: -3000}, {X: 3000, Y: 3000}, {X: -3000, Y: 3000}}}
	area, _ := calcCutArea(clipper.IntPoint{X: 0, Y: 0}, clipper.IntPoint{X: 200, Y: 0}, 1000, cleared)
	if math.Abs(area) > 2.0 {
		t.Fatalf("cut area over an already-cleared region = %g, want ~0", area)
	}
}

func TestCalcCutAreaPartiallyCleared(t *testing.T) {
	// A cleared half-plane (y >= 0) removes roughly the top half of the crescent; the remaining
	// cut area should be positive but well under the full crescent.
	const r, d = 1000.0, 200.0
	cleared := clipper.Paths{{{X: -3000, Y: 0}, {X: 3000, Y: 0}, {X: 3000, Y: 3000}, {X: -3000, Y: 3000}}}
	area, _ := calcCutArea(clipper.IntPoint{X: 0, Y: 0}, clipper.IntPoint{X: int64(d), Y: 0}, int64(r), cleared)
	full := crescentArea(r, d)
	if area <= 0 || area >= full {
		t.Fatalf("partially-cleared cut area = %g, want in (0, %g)", area, full)
	}
}
