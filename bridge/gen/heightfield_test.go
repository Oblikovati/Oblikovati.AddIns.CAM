// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"math"
	"testing"
)

// coneField builds a cone height field Z = peak − r (r = distance from centre) on a grid over
// [-half,half]² — so a contour at level L is a circle of radius (peak − L).
func coneField(half, step, peak float64) *Heightfield {
	n := int(2*half/step) + 1
	h := &Heightfield{X0: -half, Y0: -half, Step: step, NX: n, NY: n, Z: make([]float64, n*n)}
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			x, y := -half+float64(i)*step, -half+float64(j)*step
			h.Z[i*n+j] = peak - math.Hypot(x, y)
		}
	}
	return h
}

// TestContourCircle slices a cone at a level and checks the contour is a closed loop on the
// expected-radius circle.
func TestContourCircle(t *testing.T) {
	h := coneField(15, 0.5, 20)
	loops := h.Contour(12) // radius 20−12 = 8
	if len(loops) != 1 {
		t.Fatalf("cone sliced at one level should give 1 loop, got %d", len(loops))
	}
	loop := loops[0]
	if len(loop) < 8 {
		t.Fatalf("contour loop too coarse: %d points", len(loop))
	}
	if quantise(loop[0]) != quantise(loop[len(loop)-1]) {
		t.Errorf("contour loop is not closed: starts %v ends %v", loop[0], loop[len(loop)-1])
	}
	for _, p := range loop {
		r := math.Hypot(p[0], p[1])
		if math.Abs(r-8) > 0.6 {
			t.Errorf("contour point %v at radius %.3f, want ~8", p, r)
		}
	}
}

// TestContourBelowAndAbove returns no loops outside the field's value range.
func TestContourBelowAndAbove(t *testing.T) {
	h := coneField(10, 1, 20)
	if loops := h.Contour(100); len(loops) != 0 {
		t.Errorf("a level above the whole field must yield no contour, got %d loops", len(loops))
	}
	if loops := h.Contour(-100); len(loops) != 0 {
		t.Errorf("a level below the whole field must yield no contour, got %d loops", len(loops))
	}
}

// TestContourSkipsNaN leaves a gap where data is missing rather than crossing it.
func TestContourSkipsNaN(t *testing.T) {
	h := coneField(10, 1, 20)
	h.Z[0] = math.NaN() // poke a hole
	// must not panic and must still contour the rest
	if loops := h.Contour(12); len(loops) == 0 {
		t.Error("a single NaN must not wipe out the whole contour")
	}
}
