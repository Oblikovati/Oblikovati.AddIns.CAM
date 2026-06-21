// SPDX-License-Identifier: GPL-2.0-only

//go:build !cgo

package adaptive

import "fmt"

// Execute is the non-cgo stub: the faithful Adaptive2d clearing solver runs on the vendored Clipper
// engine, which is compiled only in cgo builds. Returning an error lets callers fall back to the
// simpler spiral generator rather than fail.
func Execute(cfg Config, stockPaths, paths, clearedPaths []DPath) ([]Output, error) {
	return nil, fmt.Errorf("adaptive.Execute requires the cgo build (vendored Clipper engine); this binary was built with CGO disabled")
}
