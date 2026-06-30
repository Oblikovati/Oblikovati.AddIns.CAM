// SPDX-License-Identifier: GPL-2.0-only

package plot

import (
	"image"
	"image/color"
	"math"

	"oblikovati.org/cam/bridge/gcode"
)

// Depth profile: a Z-against-progress chart of a toolpath, the standard way to read a drilling
// cycle's motion. A single plunge is one descending stroke; a G83/G73 peck cycle is a woodpecker —
// stepping down and retracting — which a carved-stock image cannot show (the removed volume is the
// same either way). The simulator validation uses this to confirm the peck animation.

var depthBackground = color.RGBA{30, 32, 38, 255} // dark slate, matching the carve renderer
var depthLine = color.RGBA{90, 200, 120, 255}     // green stroke
var depthZero = color.RGBA{70, 74, 86, 255}       // the Z=0 (stock top) reference line

// RenderDepthProfile draws a size×size chart of each point's Z (vertical, +Z up) against its index
// along the path (horizontal). Fewer than two points renders a blank frame.
func RenderDepthProfile(points []gcode.Vector3, size int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	fill(img, depthBackground)
	if len(points) < 2 {
		return img
	}
	tf := newTransform(depthBox(points), size)
	drawZeroLine(img, tf, len(points))
	for i := 0; i+1 < len(points); i++ {
		line(img, tf.px(float64(i), points[i].Z), tf.px(float64(i+1), points[i+1].Z), depthLine)
	}
	return img
}

// depthBox is the world box over (index, Z): the full index span and the Z range (including 0 so the
// stock top is always in frame), padded.
func depthBox(points []gcode.Vector3) box {
	b := box{0, 0, float64(len(points) - 1), 0}
	for _, p := range points {
		b.minY, b.maxY = math.Min(b.minY, p.Z), math.Max(b.maxY, p.Z)
	}
	return b.pad(1)
}

// drawZeroLine marks the Z=0 stock-top reference across the chart.
func drawZeroLine(img *image.RGBA, tf transform, n int) {
	line(img, tf.px(0, 0), tf.px(float64(n-1), 0), depthZero)
}
