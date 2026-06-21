// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

package bridge

import "testing"

// TestVCarveOpExecute checks a V-carve op rides the medial axis, deepening where the region is wider,
// and reports the V-Carve kind. It needs the cgo Voronoi engine, hence the build tag.
func TestVCarveOpExecute(t *testing.T) {
	op := &VCarveOp{
		OpBase:    OpBase{OpLabel: "V-Carve", IsActive: true, ClearanceHeight: 15, SafeHeight: 2, StartDepth: 0, FinalDepth: -100},
		ToolAngle: 90, Boundary: squarePoly(40),
	}
	path, err := op.Execute(millJob(40))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var minZ float64
	seen := false
	for _, c := range path.Commands {
		if z, ok := c.Params["Z"]; ok && c.Name == "G1" {
			if !seen || z < minZ {
				minZ = z
			}
			seen = true
		}
	}
	if !seen {
		t.Fatal("V-carve should produce cutting moves")
	}
	// A 40mm bit on a 40mm square reaches the centre clearance of 20mm: the deepest cut is ~-20.
	if minZ > -19 {
		t.Errorf("V-carve deepest cut = %g, want about -20 (the square's centre clearance)", minZ)
	}
	if operationKind(op) != "V-Carve" {
		t.Errorf("operationKind = %q, want V-Carve", operationKind(op))
	}

	noBoundary := &VCarveOp{OpBase: OpBase{OpLabel: "V", IsActive: true}, ToolAngle: 90}
	if _, err := noBoundary.Execute(millJob(40)); err == nil {
		t.Error("a V-carve with no boundary must error")
	}
}
