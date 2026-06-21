// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"math"
	"testing"
)

// These mirror the upstream V-carve depth oracle (CAMTests/TestPathVcarve.py test00–test13): the
// same V-bit (diameter, included angle, tip diameter) and Z range must yield the same start, stop,
// and scale. scale = 1/tan(halfAngle): a 90° bit gives 1, a 60° bit √3, a 45° bit 1+√2.
func TestVcarveGeometryFromToolOracle(t *testing.T) {
	const scale60 = 1.7320508 // √3
	const scale45 = 2.4142136 // 1 + √2
	cases := []struct {
		name                           string
		dia, angle, tip, zStart, zFin  float64
		wantStart, wantStop, wantScale float64
	}{
		{"90deg", 10, 90, 0, 0, -10, 0, -5, 1},
		{"90deg limited", 10, 90, 0, 0, -3, 0, -3, 1},
		{"60deg", 10, 60, 0, 0, -10, 0, -5 * scale60, scale60},
		{"60deg limited", 10, 60, 0, 0, -3, 0, -3, scale60},
		{"90deg tip", 10, 90, 2, 0, -10, 1, -4, 1},
		{"90deg tip limited", 10, 90, 2, 0, -3, 1, -3, 1},
		{"45deg tip", 10, 45, 2, 0, -10, scale45, -5*scale45 + scale45, scale45},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := vcarveGeometryFromTool(c.dia, c.angle, c.tip, c.zStart, c.zFin, 0)
			assertRoughly(t, "start", g.start, c.wantStart)
			assertRoughly(t, "stop", g.stop, c.wantStop)
			assertRoughly(t, "scale", g.scale, c.wantScale)
		})
	}
}

// TestVcarveDepthForClearance checks the depth model: with a 90° bit (scale 1) starting at 0, a
// medial point with 3mm clearance carves to z=-3, and the depth is clamped to the stop plane.
func TestVcarveDepthForClearance(t *testing.T) {
	g := vcarveGeometryFromTool(10, 90, 0, 0, -10, 0) // start 0, stop -5, scale 1
	if got := g.depthForClearance(3); math.Abs(got-(-3)) > 1e-6 {
		t.Errorf("depth at 3mm clearance = %.4f, want -3", got)
	}
	if got := g.depthForClearance(8); math.Abs(got-(-5)) > 1e-6 {
		t.Errorf("depth at 8mm clearance = %.4f, want -5 (clamped to stop)", got)
	}
}

func assertRoughly(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-4 {
		t.Errorf("%s = %.6f, want %.6f", label, got, want)
	}
}
