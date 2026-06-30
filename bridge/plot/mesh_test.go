// SPDX-License-Identifier: GPL-2.0-only

package plot

import (
	"image"
	"testing"
)

// cubeMesh is a unit cube as an indexed triangle mesh (12 triangles), for renderer tests.
func cubeMesh() ([]float64, []int) {
	c := []float64{
		0, 0, 0, 1, 0, 0, 1, 1, 0, 0, 1, 0, // bottom z=0
		0, 0, 1, 1, 0, 1, 1, 1, 1, 0, 1, 1, // top z=1
	}
	idx := []int{
		0, 2, 1, 0, 3, 2, // bottom
		4, 5, 6, 4, 6, 7, // top
		0, 1, 5, 0, 5, 4, // -Y
		2, 3, 7, 2, 7, 6, // +Y
		1, 2, 6, 1, 6, 5, // +X
		0, 4, 7, 0, 7, 3, // -X
	}
	return c, idx
}

// TestRenderMeshProducesImage checks the renderer returns an image of the requested size with the
// shape drawn over the background (some pixels differ from the corners).
func TestRenderMeshProducesImage(t *testing.T) {
	coords, indices := cubeMesh()
	img := RenderMesh(coords, indices, 200)
	b := img.Bounds()
	if b.Dx() != 200 || b.Dy() != 200 {
		t.Fatalf("size = %dx%d, want 200x200", b.Dx(), b.Dy())
	}
	if !hasForeground(img) {
		t.Error("nothing drawn — image is uniform background")
	}
}

// TestRenderEmptyMeshIsBlank checks an empty mesh renders without panicking (a blank image).
func TestRenderEmptyMeshIsBlank(t *testing.T) {
	img := RenderMesh(nil, nil, 64)
	if img.Bounds().Dx() != 64 {
		t.Errorf("size = %d, want 64", img.Bounds().Dx())
	}
}

// hasForeground reports whether any pixel differs from the top-left (background) pixel.
func hasForeground(img image.Image) bool {
	bg := img.At(0, 0)
	br, bgg, bb, _ := bg.RGBA()
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if r != br || g != bgg || bl != bb {
				return true
			}
		}
	}
	return false
}
