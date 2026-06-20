// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "oblikovati.org/cam/bridge/ocl"

// Surfacer drops a ball-nose cutter over a triangle mesh along parallel scan lines, returning
// the surface-following cutter-location rows. It is the seam between the engine and the 3D
// drop-cutter engine: the default implementation is OpenCAMLib (bridge/ocl), and tests inject a
// fake so the engine's mesh-gathering and toolpath assembly are testable without cgo.
type Surfacer interface {
	DropCutter(tris []ocl.Triangle, diameter, length, minZ, sampling float64, lines []ocl.ScanLine) ([][]ocl.Point3, error)
}

// oclSurfacer is the production Surfacer backed by the vendored OpenCAMLib drop-cutter.
type oclSurfacer struct{}

// DropCutter forwards to OpenCAMLib (or its non-cgo stub error).
func (oclSurfacer) DropCutter(tris []ocl.Triangle, diameter, length, minZ, sampling float64, lines []ocl.ScanLine) ([][]ocl.Point3, error) {
	return ocl.DropCutter(tris, diameter, length, minZ, sampling, lines)
}

// WithSurfacer overrides the engine's drop-cutter backend (used by tests). Returns the engine
// for chaining.
func (e *Engine) WithSurfacer(s Surfacer) *Engine {
	e.surfacer = s
	return e
}
