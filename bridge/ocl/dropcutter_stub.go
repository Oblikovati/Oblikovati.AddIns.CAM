// SPDX-License-Identifier: GPL-2.0-only

//go:build !cgo

package ocl

import "fmt"

// DropCutter is the non-cgo stub: 3D surface finishing needs the vendored OpenCAMLib, which is
// only compiled in cgo builds. Returning an error keeps the add-in buildable and testable with
// CGO disabled (the CI test matrix) while the real implementation ships in the cgo build.
func DropCutter(tris []Triangle, diameter, length, minZ, sampling float64, lines []ScanLine) ([][]Point3, error) {
	return nil, fmt.Errorf("ocl: 3D surface finishing requires the cgo build (vendored OpenCAMLib); this binary was built with CGO disabled")
}
