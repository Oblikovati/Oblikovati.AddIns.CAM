// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"fmt"
	"math"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/geom2d"
	"oblikovati.org/cam/bridge/voronoi"
)

// vcarveScale maps millimetres into the Voronoi engine's integer plane. 1000 keeps sub-micron
// precision while staying within the engine's 32-bit coordinate range for parts up to ~2 m.
const vcarveScale = 1000.0

// VCarveParams configure V-carving with a V-bit. The bit rides the region's true medial axis (its
// spine), cut deeper where the region is wider: the depth at each point is the radius of the largest
// inscribed circle there, scaled by 1/tan(halfAngle), so the groove forms a V cross-section —
// shallow where the walls crowd, deepest along the spine. Used for engraved lettering and reliefs.
type VCarveParams struct {
	ToolAngleDeg float64 // included angle of the V-bit (degrees); <=0 → 90°
	ToolDiameter float64 // mm — the bit's full cutting diameter (sets the deepest reachable cut)
	TipDiameter  float64 // mm — the flat tip diameter (0 for a sharp point); shifts the carve up
	FinalDepth   float64 // mm — a hard lower Z limit the carve may not pass
	StepDown     float64 // mm — max depth per roughing pass (0 → a single pass to the limit)
}

// GenerateVCarve carves the region inside the boundary along its medial axis. It builds the segment
// Voronoi diagram of the boundary, keeps the interior medial edges, and rides each as a continuous
// move whose Z follows the local clearance through the V-bit depth model. Unlike an offset-contour
// approximation, the medial axis is the exact locus of maximum inscribed circles, so the V-bit
// tracks the true spine and never over- or under-cuts where crowded corners meet.
func GenerateVCarve(boundary geom2d.Polygon, top float64, feeds Feeds, p VCarveParams) ([]gcode.Command, error) {
	if p.ToolDiameter <= 0 {
		return nil, fmt.Errorf("v-carve needs a positive tool diameter, got %g", p.ToolDiameter)
	}
	if len(boundary) < 3 {
		return nil, fmt.Errorf("v-carve boundary needs at least 3 points, got %d", len(boundary))
	}
	angle := p.ToolAngleDeg
	if angle <= 0 {
		angle = 90
	}
	geom := vcarveGeometryFromTool(p.ToolDiameter, angle, p.TipDiameter, top, p.FinalDepth, p.StepDown)

	medial, err := voronoi.MedialAxis(boundarySegments(boundary))
	if err != nil {
		return nil, fmt.Errorf("v-carve medial axis: %w", err)
	}

	var cmds []gcode.Command
	for _, wire := range chainMedialWires(interiorMedialEdges(medial, boundary)) {
		cmds = append(cmds, walkMedialWire(wire, geom, feeds)...)
	}
	return cmds, nil
}

// boundarySegments turns the closed boundary polygon into the scaled-integer segments the Voronoi
// engine consumes.
func boundarySegments(poly geom2d.Polygon) []voronoi.Segment {
	n := len(poly)
	segs := make([]voronoi.Segment, n)
	for i := 0; i < n; i++ {
		a, b := poly[i], poly[(i+1)%n]
		segs[i] = voronoi.Segment{A: scalePoint(a), B: scalePoint(b)}
	}
	return segs
}

// scalePoint maps a millimetre point into the Voronoi integer plane.
func scalePoint(p geom2d.Point2) voronoi.Point {
	return voronoi.Point{X: int64(math.Round(p.X * vcarveScale)), Y: int64(math.Round(p.Y * vcarveScale))}
}

// medialPoint is a medial-axis vertex in millimetres with its clearance (max inscribed circle radius).
type medialPoint struct {
	X, Y, Clearance float64
}

// medialSegment is one interior medial edge in millimetres.
type medialSegment struct {
	A, B medialPoint
}

// interiorMedialEdges keeps the medial edges whose midpoint lies inside the boundary (the Voronoi of
// the boundary segments also fills the exterior, which the carve must ignore) and converts them from
// the scaled plane to millimetres.
func interiorMedialEdges(edges []voronoi.MedialEdge, boundary geom2d.Polygon) []medialSegment {
	var out []medialSegment
	for _, e := range edges {
		a := medialPoint{X: e.A.X / vcarveScale, Y: e.A.Y / vcarveScale, Clearance: e.A.Clearance / vcarveScale}
		b := medialPoint{X: e.B.X / vcarveScale, Y: e.B.Y / vcarveScale, Clearance: e.B.Clearance / vcarveScale}
		mid := geom2d.Point2{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
		if boundary.Contains(mid) {
			out = append(out, medialSegment{A: a, B: b})
		}
	}
	return out
}

// medialWire is a connected chain of medial points the V-bit rides in one continuous move.
type medialWire []medialPoint

// chainMedialWires links the interior medial edges end-to-end into wires, greedily extending each
// wire from its tail through shared endpoints so the bit stays down along a connected spine instead
// of retracting between every edge. A wire ends at a junction or a free end.
func chainMedialWires(segs []medialSegment) []medialWire {
	used := make([]bool, len(segs))
	adj := make(map[[2]int64][]int, 2*len(segs))
	for i, s := range segs {
		adj[gridKey(s.A)] = append(adj[gridKey(s.A)], i)
		adj[gridKey(s.B)] = append(adj[gridKey(s.B)], i)
	}
	var wires []medialWire
	for start := range segs {
		if used[start] {
			continue
		}
		used[start] = true
		wire := medialWire{segs[start].A, segs[start].B}
		for {
			tail := wire[len(wire)-1]
			next := nextUnused(adj[gridKey(tail)], used)
			if next < 0 {
				break
			}
			used[next] = true
			wire = append(wire, medialFarEndpoint(segs[next], tail))
		}
		wires = append(wires, wire)
	}
	return wires
}

// nextUnused returns the first not-yet-used edge index in the list, or -1.
func nextUnused(candidates []int, used []bool) int {
	for _, j := range candidates {
		if !used[j] {
			return j
		}
	}
	return -1
}

// medialFarEndpoint returns the endpoint of seg that is not the shared point.
func medialFarEndpoint(seg medialSegment, shared medialPoint) medialPoint {
	if gridKey(seg.A) == gridKey(shared) {
		return seg.B
	}
	return seg.A
}

// gridKey snaps a medial point to the scaled-integer grid for endpoint matching.
func gridKey(p medialPoint) [2]int64 {
	return [2]int64{int64(math.Round(p.X * vcarveScale)), int64(math.Round(p.Y * vcarveScale))}
}

// walkMedialWire emits the moves to ride one medial wire: rapid above the start, plunge to its carve
// depth, then feed along the wire with Z following the local clearance, and retract.
func walkMedialWire(w medialWire, geom vcarveGeometry, feeds Feeds) []gcode.Command {
	if len(w) < 2 {
		return nil
	}
	cmds := []gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"Z": feeds.ClearanceZ}),
		gcode.NewCommand("G0", map[string]float64{"X": w[0].X, "Y": w[0].Y}),
		gcode.NewCommand("G1", map[string]float64{"Z": geom.depthForClearance(w[0].Clearance), "F": feeds.Vert}),
	}
	for _, pt := range w[1:] {
		cmds = append(cmds, gcode.NewCommand("G1", map[string]float64{
			"X": pt.X, "Y": pt.Y, "Z": geom.depthForClearance(pt.Clearance), "F": feeds.Horiz,
		}))
	}
	return append(cmds, gcode.NewCommand("G0", map[string]float64{"Z": feeds.ClearanceZ}))
}
