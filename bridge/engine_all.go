// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/ocl"
)

// RunAllJobsOnHost generates one program that runs several operations on different tools —
// drilling the holes (drill), contouring the outline (end mill), and finishing the surface
// (ball-nose) — so the posted G-code carries the tool changes between them. It is the
// demonstration that the job's tool controllers are selected per operation by cutter shape.
func (e *Engine) RunAllJobsOnHost(bodyIndex int) (*JobResult, error) {
	tris, stock, err := e.meshAndStock(bodyIndex)
	if err != nil {
		return nil, err
	}
	boundary, _, err := e.contourAndStock(bodyIndex)
	if err != nil {
		return nil, err
	}
	holes, _, err := e.detectHolesAndStock(bodyIndex)
	if err != nil {
		return nil, err
	}

	job := e.newMillJob(bodyIndex, stock)
	cut := e.cutting()
	var ops []Operation
	if len(holes) > 0 {
		ops = append(ops, &DrillingOp{OpBase: e.drillEnvelope(stock, indexForShape(job.Tools, "drill")), Holes: holes})
	}
	ops = append(ops, &ProfileOp{
		OpBase: e.millEnvelope("Profile", stock), Side: "outside", Climb: true, StepDown: cut.StepDown, Boundary: boundary,
	})
	surface, err := e.surfaceOp(job, stock, tris, cut)
	if err != nil {
		return nil, err
	}
	ops = append(ops, surface)
	job.Operations = ops

	return e.postPreviewResult(job, fmt.Sprintf("generated all operations (%d ops over %d tools)", len(ops), distinctTools(ops)))
}

// surfaceOp builds the 3D finishing operation (ball-nose) for a combined job, running the
// drop-cutter over the mesh.
func (e *Engine) surfaceOp(job *Job, stock Stock, tris []ocl.Triangle, cut cutSettings) (*SurfaceOp, error) {
	ball := indexForShape(job.Tools, "ballend")
	diameter := job.Tools[ball].Tool.Diameter
	stepOver := passSpacing(cut, diameter, surfStepOver)
	lines := scanLines(stock, stepOver)
	rows, err := e.surfacer.DropCutter(tris, diameter, ballCutterLength, stock.BottomZ(), surfSampling, lines)
	if err != nil {
		return nil, fmt.Errorf("drop-cutter over %d triangles / %d passes: %w", len(tris), len(lines), err)
	}
	env := e.millEnvelope("Surface", stock)
	env.ToolController = ball
	return &SurfaceOp{OpBase: env, StepOver: stepOver, Sampling: surfSampling, Zigzag: true, Rows: toVectorRows(rows)}, nil
}

// distinctTools counts the distinct tool-controller indices the operations use — how many tool
// changes the program will carry.
func distinctTools(ops []Operation) int {
	seen := map[int]bool{}
	for _, op := range ops {
		seen[op.ToolControllerIndex()] = true
	}
	return len(seen)
}
