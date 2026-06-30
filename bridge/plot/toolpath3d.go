// SPDX-License-Identifier: GPL-2.0-only

package plot

import (
	"image"
	"math"

	"oblikovati.org/cam/bridge/gcode"
)

// 3-D toolpath backplot: the simulator's playback overlay drawn statically — the motion path in the
// same isometric view as the carved stock, with cutting moves green and rapids blue (the colours the
// live viewport overlay uses). It lets the overlay's rapid/feed colouring be validated as an image,
// off the live head.

// RenderToolpath3D draws the path's motion as coloured line segments in a size×size isometric frame:
// cutting moves (G1/G2/G3) green, rapids (G0) blue. A path with fewer than two motion points renders
// blank.
func RenderToolpath3D(path gcode.Path, size int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	fill(img, depthBackground)
	coords, feed := motionXYZ(path)
	if len(coords) < 6 {
		return img
	}
	cam := isoCamera()
	pv := projectVertices(coords, cam)
	tf := fitProjected(pv, size)
	for i := 0; i+1 < len(pv); i++ {
		col := depthRapid
		if feed[i+1] {
			col = depthFeed
		}
		line(img, roundPt(tf.at(pv[i])), roundPt(tf.at(pv[i+1])), col)
	}
	return img
}

// motionXYZ walks the path into one (x,y,z) point per motion command (xyz triples, mm) plus whether
// each was reached by a cutting move rather than a rapid.
func motionXYZ(path gcode.Path) ([]float64, []bool) {
	var coords []float64
	var feed []bool
	var x, y, z float64
	for _, c := range path.Commands {
		if v, ok := c.Params["X"]; ok {
			x = v
		}
		if v, ok := c.Params["Y"]; ok {
			y = v
		}
		if v, ok := c.Params["Z"]; ok {
			z = v
		}
		if isMotion(c.Name) {
			coords = append(coords, x, y, z)
			feed = append(feed, c.Name != "G0" && c.Name != "G00")
		}
	}
	return coords, feed
}

// roundPt rounds a sub-pixel projected point to an integer pixel.
func roundPt(s screenPt) pt {
	return pt{int(math.Round(s.x)), int(math.Round(s.y))}
}
