// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
)

// MaterialMesh runs the full material-removal pipeline for a job headlessly and returns the carved
// stock as a surface mesh: regenerate the (host-free) toolpaths, voxelise the stock, sweep every
// cut, and extract the solid/empty boundary. Coordinates are millimetre xyz triples; res is the
// voxel cell size used. This is the simulator's carve without the live host — used by the screenshot
// harness and by callers that want the finished part geometry rather than an animation.
func MaterialMesh(job *Job) (coords []float64, indices []int, res float64, err error) {
	results, err := job.GenerateAll()
	if err != nil {
		return nil, nil, 0, fmt.Errorf("generate toolpaths: %w", err)
	}
	stock := job.Stock
	cuts := flattenCuts(results, stock.Max.Z-stock.Min.Z)
	if len(cuts) == 0 {
		return nil, nil, 0, fmt.Errorf("job produced no cutting moves to remove material")
	}
	res = chooseVoxelRes(stock.Min, stock.Max, minCutterRadius(cuts))
	g := NewVoxelGrid(stock.Min, stock.Max, res)
	for _, c := range cuts {
		sweepSegment(g, c.cutter, c.from, c.to)
	}
	coords, indices = voxelSurfaceMesh(g)
	return coords, indices, res, nil
}

// MaterialToolpath returns the simulator's playback polyline (mm) for a job: the cutter-tip path
// after canned-cycle expansion — exactly the points the material view animates through. Useful for
// validating motion headlessly, e.g. the peck descent of deep-hole drilling.
func MaterialToolpath(job *Job) ([]gcode.Vector3, error) {
	results, err := job.GenerateAll()
	if err != nil {
		return nil, fmt.Errorf("generate toolpaths: %w", err)
	}
	cuts := flattenCuts(results, job.Stock.Max.Z-job.Stock.Min.Z)
	if len(cuts) == 0 {
		return nil, fmt.Errorf("job produced no cutting moves")
	}
	return cutPoints(cuts), nil
}

// MaterialPath returns the job's expanded motion program — every active operation's path with its
// canned cycles unfolded into explicit G0/G1 moves. Unlike MaterialToolpath it keeps the rapid/feed
// distinction, so a backplot can colour rapids and cuts (e.g. to show a tap threading back out).
func MaterialPath(job *Job) (gcode.Path, error) {
	results, err := job.GenerateAll()
	if err != nil {
		return gcode.Path{}, fmt.Errorf("generate toolpaths: %w", err)
	}
	var cmds []gcode.Command
	for _, r := range results {
		cmds = append(cmds, gcode.ExpandCannedCycles(r.Path).Commands...)
	}
	if len(cmds) == 0 {
		return gcode.Path{}, fmt.Errorf("job produced no motion")
	}
	return gcode.NewPath(cmds), nil
}
