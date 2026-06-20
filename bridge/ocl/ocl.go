// SPDX-License-Identifier: GPL-2.0-only

// Package ocl is the project's thin Go interface over OpenCAMLib's drop-cutter — the engine
// behind 3D surface finishing. The actual C++ library (LGPL-2.1, COPYING.opencamlib) is
// vendored here as a Boost-free subset and compiled by cgo; a non-cgo stub keeps the rest of
// the add-in buildable and testable without a C toolchain. Lengths are millimetres.
package ocl

// Triangle is one mesh facet (three xyz corners) of the model surface.
type Triangle struct{ A, B, C [3]float64 }

// ScanLine is one XY parallel pass the cutter is dropped along.
type ScanLine struct{ X0, Y0, X1, Y1 float64 }

// Point3 is a cutter-location point (the tool tip) the drop-cutter returns.
type Point3 struct{ X, Y, Z float64 }
