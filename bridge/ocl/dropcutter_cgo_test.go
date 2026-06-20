// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

package ocl

import (
	"math"
	"testing"
)

// tentMesh is a ridge along X peaking at z=5 on the y=0 line, sloping to z=0 at y=±10, over
// x∈[-10,10]. A ball cutter dropped across it should follow the slopes and peak near the ridge.
func tentMesh() []Triangle {
	a, b := [3]float64{-10, 0, 5}, [3]float64{10, 0, 5}
	sw, se := [3]float64{-10, -10, 0}, [3]float64{10, -10, 0}
	nw, ne := [3]float64{-10, 10, 0}, [3]float64{10, 10, 0}
	return []Triangle{{a, b, sw}, {b, se, sw}, {a, nw, b}, {b, nw, ne}}
}

// TestDropCutterFollowsRidge drops a ball cutter across the tent and checks the returned CL
// points rise to ~5 at the ridge and the rows match the scan lines.
func TestDropCutterFollowsRidge(t *testing.T) {
	lines := []ScanLine{{X0: 0, Y0: -8, X1: 0, Y1: 8}, {X0: 5, Y0: -8, X1: 5, Y1: 8}}
	rows, err := DropCutter(tentMesh(), 6, 20, -5, 2, lines)
	if err != nil {
		t.Fatalf("DropCutter: %v", err)
	}
	if len(rows) != len(lines) {
		t.Fatalf("got %d rows, want %d", len(rows), len(lines))
	}
	for r, row := range rows {
		if len(row) < 2 {
			t.Fatalf("row %d has %d points, want a sampled pass", r, len(row))
		}
		peak := math.Inf(-1)
		for _, p := range row {
			peak = math.Max(peak, p.Z)
		}
		if peak < 4.0 || peak > 5.2 {
			t.Errorf("row %d peak z = %.3f, want ~5 (ball tip riding the z=5 ridge)", r, peak)
		}
	}
}

// TestDropCutterRejectsBadInput covers the guards.
func TestDropCutterRejectsBadInput(t *testing.T) {
	if _, err := DropCutter(nil, 6, 20, -5, 2, []ScanLine{{}}); err == nil {
		t.Error("empty mesh must error")
	}
	if _, err := DropCutter(tentMesh(), 6, 20, -5, 2, nil); err == nil {
		t.Error("no scan lines must error")
	}
	if _, err := DropCutter(tentMesh(), 0, 20, -5, 2, []ScanLine{{}}); err == nil {
		t.Error("non-positive diameter must error")
	}
}
