// SPDX-License-Identifier: GPL-2.0-only

package plot

import (
	"image"
	"image/color"
	"math"

	"oblikovati.org/cam/bridge/gcode"
)

// Depth profile: a Z-against-progress chart of a toolpath, the standard way to read a drilling or
// tapping cycle's motion. Rapids (G0) are drawn blue and cutting moves (G1/G2/G3) green — the CAM
// backplot convention — so a cycle's character reads at a glance: a drill plunges (green) and rapids
// out (blue); a peck cycle is a green/blue woodpecker; a tap threads in AND out under feed (green
// both ways). The removed volume is identical across these, so a carved-stock image cannot show it.

var depthBackground = color.RGBA{30, 32, 38, 255} // dark slate, matching the carve renderer
var depthFeed = color.RGBA{90, 200, 120, 255}     // cutting move (G1/G2/G3)
var depthRapid = color.RGBA{90, 130, 220, 255}    // rapid move (G0)
var depthZero = color.RGBA{70, 74, 86, 255}       // the Z=0 (stock top) reference line

// RenderDepthProfile draws a size×size chart of each motion command's Z (vertical, +Z up) against
// its index along the path (horizontal), colouring rapids and feeds distinctly. A path with fewer
// than two moves renders a blank frame.
func RenderDepthProfile(path gcode.Path, size int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	fill(img, depthBackground)
	zs, feed := motionDepths(path)
	if len(zs) < 2 {
		return img
	}
	tf := newTransform(depthBox(zs), size)
	drawZeroLine(img, tf, len(zs))
	for i := 0; i+1 < len(zs); i++ {
		col := depthRapid
		if feed[i+1] {
			col = depthFeed
		}
		line(img, tf.px(float64(i), zs[i]), tf.px(float64(i+1), zs[i+1]), col)
	}
	return img
}

// motionDepths walks the path into one (Z, isFeed) sample per motion command, with Z sticky across
// commands that omit it. isFeed marks a cutting move (anything but a G0 rapid).
func motionDepths(path gcode.Path) ([]float64, []bool) {
	var zs []float64
	var feed []bool
	z := 0.0
	for _, c := range path.Commands {
		if v, ok := c.Params["Z"]; ok {
			z = v
		}
		if isMotion(c.Name) {
			zs = append(zs, z)
			feed = append(feed, c.Name != "G0" && c.Name != "G00")
		}
	}
	return zs, feed
}

// isMotion reports whether a G-word is a rapid or feed move (incl. leading-zero forms).
func isMotion(name string) bool {
	switch name {
	case "G0", "G1", "G2", "G3", "G00", "G01", "G02", "G03":
		return true
	}
	return false
}

// depthBox is the world box over (index, Z): the full index span and the Z range (including 0 so the
// stock top stays in frame), padded.
func depthBox(zs []float64) box {
	b := box{0, 0, float64(len(zs) - 1), 0}
	for _, z := range zs {
		b.minY, b.maxY = math.Min(b.minY, z), math.Max(b.maxY, z)
	}
	return b.pad(1)
}

// drawZeroLine marks the Z=0 stock-top reference across the chart.
func drawZeroLine(img *image.RGBA, tf transform, n int) {
	line(img, tf.px(0, 0), tf.px(float64(n-1), 0), depthZero)
}
