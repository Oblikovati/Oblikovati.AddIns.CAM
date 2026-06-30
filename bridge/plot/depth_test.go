// SPDX-License-Identifier: GPL-2.0-only

package plot

import (
	"image"
	"image/color"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// peckPath is a tiny rapid/feed program (a two-peck descent) for renderer tests.
func peckPath() gcode.Path {
	return gcode.NewPath([]gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"Z": 2}),
		gcode.NewCommand("G1", map[string]float64{"Z": -1}),
		gcode.NewCommand("G0", map[string]float64{"Z": 2}),
		gcode.NewCommand("G1", map[string]float64{"Z": -4}),
		gcode.NewCommand("G0", map[string]float64{"Z": 2}),
	})
}

// TestRenderDepthProfileDraws checks the depth chart is the requested size and draws the path over
// the background.
func TestRenderDepthProfileDraws(t *testing.T) {
	img := RenderDepthProfile(peckPath(), 240)
	if img.Bounds().Dx() != 240 || img.Bounds().Dy() != 240 {
		t.Fatalf("size = %dx%d, want 240x240", img.Bounds().Dx(), img.Bounds().Dy())
	}
	if !hasForeground(img) {
		t.Error("depth profile drew nothing")
	}
}

// TestRenderDepthProfileColoursMoves checks rapids and feeds are drawn in different colours.
func TestRenderDepthProfileColoursMoves(t *testing.T) {
	img := RenderDepthProfile(peckPath(), 240)
	if !usesColor(img, depthFeed) || !usesColor(img, depthRapid) {
		t.Error("expected both feed and rapid colours in the chart")
	}
}

// TestRenderDepthProfileEmpty checks a too-short path renders a blank image without panicking.
func TestRenderDepthProfileEmpty(t *testing.T) {
	short := gcode.NewPath([]gcode.Command{gcode.NewCommand("G1", map[string]float64{"Z": 1})})
	if img := RenderDepthProfile(short, 80); img.Bounds().Dx() != 80 {
		t.Errorf("size = %d, want 80", img.Bounds().Dx())
	}
}

// usesColor reports whether any pixel matches c.
func usesColor(img image.Image, c color.RGBA) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			if uint8(r>>8) == c.R && uint8(g>>8) == c.G && uint8(bl>>8) == c.B && uint8(a>>8) == c.A {
				return true
			}
		}
	}
	return false
}
