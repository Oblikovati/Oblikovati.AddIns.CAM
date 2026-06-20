// SPDX-License-Identifier: GPL-2.0-only

//go:build !cgo

package ocl

import "testing"

// TestDropCutterStubErrors confirms the non-cgo build reports that 3D surfacing needs cgo,
// rather than silently producing no toolpath.
func TestDropCutterStubErrors(t *testing.T) {
	rows, err := DropCutter([]Triangle{{}}, 6, 20, -5, 1, []ScanLine{{}})
	if err == nil {
		t.Fatal("the non-cgo stub must return an error")
	}
	if rows != nil {
		t.Errorf("stub must return no rows, got %d", len(rows))
	}
}
