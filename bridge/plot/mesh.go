// SPDX-License-Identifier: GPL-2.0-only

package plot

import (
	"image"
	"image/color"
	"math"
)

// Software isometric mesh renderer for the CAM material-removal screenshot harness: it draws an
// indexed triangle mesh (the carved voxel stock) as a shaded solid, resolving occlusion with a
// per-pixel depth buffer and shading each face by its normal. No GPU or host — it runs anywhere a
// PNG can be written, so the carve can be validated visually in CI or a one-shot command.

var meshBackground = color.RGBA{30, 32, 38, 255} // dark slate, so the tan stock reads clearly
var meshBase = vec3{0.72, 0.66, 0.5}             // stock colour, matching the live simulator mesh

// RenderMesh draws coords (mm xyz triples) / indices (triangle vertices) as a size×size isometric
// image. An empty mesh yields a blank background.
func RenderMesh(coords []float64, indices []int, size int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	fill(img, meshBackground)
	if len(indices) < 3 {
		return img
	}
	cam := isoCamera()
	pv := projectVertices(coords, cam)
	tf := fitProjected(pv, size)
	zb := newZBuffer(size)
	for t := 0; t+2 < len(indices); t += 3 {
		drawTriangle(img, zb, coords, pv, tf, cam, indices[t], indices[t+1], indices[t+2])
	}
	return img
}

// vec3 is a minimal 3-vector for the renderer's projection and shading maths.
type vec3 struct{ x, y, z float64 }

func (a vec3) sub(b vec3) vec3   { return vec3{a.x - b.x, a.y - b.y, a.z - b.z} }
func (a vec3) dot(b vec3) float64 { return a.x*b.x + a.y*b.y + a.z*b.z }
func (a vec3) cross(b vec3) vec3 {
	return vec3{a.y*b.z - a.z*b.y, a.z*b.x - a.x*b.z, a.x*b.y - a.y*b.x}
}

func (a vec3) normalized() vec3 {
	l := math.Sqrt(a.dot(a))
	if l == 0 {
		return a
	}
	return vec3{a.x / l, a.y / l, a.z / l}
}

// camera is an orthographic isometric view: an orthonormal screen basis (right, up) plus the view
// direction fwd (target→eye, so larger fwd·p is nearer the eye) and a light direction.
type camera struct{ right, up, fwd, light vec3 }

// isoCamera looks down at 30° elevation from a 45° azimuth — a standard CAD three-quarter view.
func isoCamera() camera {
	az, el := 45*math.Pi/180, 30*math.Pi/180
	fwd := vec3{math.Cos(el) * math.Cos(az), math.Cos(el) * math.Sin(az), math.Sin(el)}
	right := fwd.cross(vec3{0, 0, 1}).normalized()
	up := right.cross(fwd).normalized()
	return camera{right, up, fwd, vec3{0.3, 0.2, 0.9}.normalized()}
}

// proj is a vertex projected to the screen plane: (sx,sy) in world units, depth along the view axis.
type proj struct{ sx, sy, depth float64 }

// projectVertices projects every mesh vertex onto the camera's screen basis.
func projectVertices(coords []float64, cam camera) []proj {
	pv := make([]proj, len(coords)/3)
	for i := range pv {
		p := vec3{coords[3*i], coords[3*i+1], coords[3*i+2]}
		pv[i] = proj{p.dot(cam.right), p.dot(cam.up), p.dot(cam.fwd)}
	}
	return pv
}

// fit maps projected world coordinates to centred pixels at a uniform scale.
type fit struct {
	scale, ox, oy float64
	size          int
}

// fitProjected computes the scale/offset that centres the projected mesh in a size×size frame with a
// margin.
func fitProjected(pv []proj, size int) fit {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, p := range pv {
		minX, maxX = math.Min(minX, p.sx), math.Max(maxX, p.sx)
		minY, maxY = math.Min(minY, p.sy), math.Max(maxY, p.sy)
	}
	margin := float64(size) * 0.08
	span := math.Max(math.Max(maxX-minX, maxY-minY), 1e-9)
	scale := (float64(size) - 2*margin) / span
	ox := (float64(size)-(maxX-minX)*scale)/2 - minX*scale
	oy := (float64(size)-(maxY-minY)*scale)/2 - minY*scale
	return fit{scale, ox, oy, size}
}

// screenPt is a pixel-space point (sub-pixel precision retained for rasterisation).
type screenPt struct{ x, y float64 }

// at maps a projected vertex to its pixel position (y flipped so +up is up on screen).
func (f fit) at(p proj) screenPt {
	return screenPt{p.sx*f.scale + f.ox, float64(f.size) - (p.sy*f.scale + f.oy)}
}

// zbuffer holds the nearest depth drawn at each pixel (larger depth = nearer the eye).
type zbuffer struct {
	d    []float64
	size int
}

func newZBuffer(size int) *zbuffer {
	d := make([]float64, size*size)
	for i := range d {
		d[i] = math.Inf(-1)
	}
	return &zbuffer{d, size}
}

// drawTriangle shades one triangle by its world normal and rasterises it into the depth buffer.
func drawTriangle(img *image.RGBA, zb *zbuffer, coords []float64, pv []proj, tf fit, cam camera, i0, i1, i2 int) {
	n := triangleNormal(coords, i0, i1, i2)
	col := faceShade(n, cam)
	a, b, c := tf.at(pv[i0]), tf.at(pv[i1]), tf.at(pv[i2])
	rasterize(img, zb, a, b, c, pv[i0].depth, pv[i1].depth, pv[i2].depth, col)
}

// triangleNormal returns the unit normal of the triangle in world space.
func triangleNormal(coords []float64, i0, i1, i2 int) vec3 {
	v := func(i int) vec3 { return vec3{coords[3*i], coords[3*i+1], coords[3*i+2]} }
	return v(i1).sub(v(i0)).cross(v(i2).sub(v(i0))).normalized()
}

// faceShade is the stock colour under ambient plus Lambert diffuse from the light.
func faceShade(n vec3, cam camera) color.RGBA {
	intensity := 0.35 + 0.65*math.Max(0, math.Abs(n.dot(cam.light)))
	return color.RGBA{chan8(meshBase.x * intensity), chan8(meshBase.y * intensity), chan8(meshBase.z * intensity), 255}
}

// chan8 converts a 0..1 intensity to an 8-bit colour channel.
func chan8(v float64) uint8 { return uint8(math.Round(math.Max(0, math.Min(1, v)) * 255)) }

// rasterize fills a triangle with depth testing, interpolating per-pixel depth by barycentric
// weights.
func rasterize(img *image.RGBA, zb *zbuffer, a, b, c screenPt, da, db, dc float64, col color.RGBA) {
	area := edge(a, b, c)
	if area == 0 {
		return
	}
	x0, x1, y0, y1 := triBounds(a, b, c, zb.size)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			p := screenPt{float64(x) + 0.5, float64(y) + 0.5}
			w0, w1, w2 := edge(b, c, p), edge(c, a, p), edge(a, b, p)
			if !inside(w0, w1, w2) {
				continue
			}
			depth := (w0*da + w1*db + w2*dc) / area
			if i := y*zb.size + x; depth > zb.d[i] {
				zb.d[i] = depth
				img.SetRGBA(x, y, col)
			}
		}
	}
}

// edge is twice the signed area of triangle (a,b,p) — positive when p is left of a→b.
func edge(a, b, p screenPt) float64 {
	return (b.x-a.x)*(p.y-a.y) - (b.y-a.y)*(p.x-a.x)
}

// inside reports whether a pixel's barycentric weights place it within the triangle (any winding).
func inside(w0, w1, w2 float64) bool {
	return (w0 >= 0 && w1 >= 0 && w2 >= 0) || (w0 <= 0 && w1 <= 0 && w2 <= 0)
}

// triBounds is the pixel bounding box of a triangle, clamped to the image.
func triBounds(a, b, c screenPt, size int) (x0, x1, y0, y1 int) {
	x0 = clampInt(int(math.Floor(math.Min(a.x, math.Min(b.x, c.x)))), size)
	x1 = clampInt(int(math.Ceil(math.Max(a.x, math.Max(b.x, c.x)))), size)
	y0 = clampInt(int(math.Floor(math.Min(a.y, math.Min(b.y, c.y)))), size)
	y1 = clampInt(int(math.Ceil(math.Max(a.y, math.Max(b.y, c.y)))), size)
	return
}

// clampInt pins a pixel index into [0,size-1].
func clampInt(v, size int) int {
	if v < 0 {
		return 0
	}
	if v > size-1 {
		return size - 1
	}
	return v
}
