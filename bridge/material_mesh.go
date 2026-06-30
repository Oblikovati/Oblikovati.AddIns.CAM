// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "fmt"

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
