// SPDX-License-Identifier: GPL-2.0-only

//go:build !cgo

package clipper

import "fmt"

// errNoCgo explains why the engine is unavailable, so callers (e.g. adaptive clearing) can fall
// back to a pure-Go strategy rather than fail. The cheap predicates in clipper.go still work in
// this build; only the heavyweight Vatti boolean/offset engine needs cgo.
const errNoCgo = "clipper: %s requires the cgo build (vendored Clipper engine); this binary was built with CGO disabled"

// Boolean is the non-cgo stub: the Vatti boolean engine is the vendored C++ compiled only in cgo
// builds. Returning an error keeps the add-in buildable and testable with CGO disabled.
func Boolean(clip ClipType, fill FillType, subjects Paths, subjClosed bool, clips Paths, returnOpen bool) (Paths, error) {
	return nil, fmt.Errorf(errNoCgo, "boolean clipping")
}

// Offset is the non-cgo stub for polygon offsetting; see Boolean.
func Offset(paths Paths, join JoinType, end EndType, delta, miterLimit, arcTolerance float64) (Paths, error) {
	return nil, fmt.Errorf(errNoCgo, "polygon offset")
}

// Simplify is the non-cgo stub for self-intersection resolution; see Boolean.
func Simplify(paths Paths, fill FillType) (Paths, error) {
	return nil, fmt.Errorf(errNoCgo, "polygon simplify")
}

// PathIntersectArea is the non-cgo stub for clipping an open path by a closed area; see Boolean.
func PathIntersectArea(subject Path, obj Paths) (Paths, error) {
	return nil, fmt.Errorf(errNoCgo, "path-area intersection")
}
