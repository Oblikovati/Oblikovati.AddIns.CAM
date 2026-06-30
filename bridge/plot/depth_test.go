// SPDX-License-Identifier: GPL-2.0-only

package plot

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// TestRenderDepthProfileDraws checks the depth chart is the requested size and draws the path over
// the background.
func TestRenderDepthProfileDraws(t *testing.T) {
	pts := []gcode.Vector3{{Z: 2}, {Z: -1}, {Z: 2}, {Z: -4}, {Z: 2}, {Z: -7}}
	img := RenderDepthProfile(pts, 240)
	if img.Bounds().Dx() != 240 || img.Bounds().Dy() != 240 {
		t.Fatalf("size = %dx%d, want 240x240", img.Bounds().Dx(), img.Bounds().Dy())
	}
	if !hasForeground(img) {
		t.Error("depth profile drew nothing")
	}
}

// TestRenderDepthProfileEmpty checks a too-short path renders a blank image without panicking.
func TestRenderDepthProfileEmpty(t *testing.T) {
	if img := RenderDepthProfile([]gcode.Vector3{{Z: 1}}, 80); img.Bounds().Dx() != 80 {
		t.Errorf("size = %d, want 80", img.Bounds().Dx())
	}
}
