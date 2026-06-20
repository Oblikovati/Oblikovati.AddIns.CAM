// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/api/wire"
)

// drillClearanceAbove / drillRetractAbove are the rapid and canned-cycle-R planes set above
// the stock top (mm): the tool rapids at clearance and the cycle retracts to the R plane.
const (
	drillClearanceAbove = 5.0
	drillRetractAbove   = 2.0
)

// detectHolesAndStock reads the body's topology + extent over the API, returning its
// cylindrical-hole drill targets and stock. Shared by the drilling and helix jobs.
func (e *Engine) detectHolesAndStock(bodyIndex int) ([]DrillTarget, Stock, error) {
	refs, err := e.api.Model().ReferenceKeys()
	if err != nil {
		return nil, Stock{}, fmt.Errorf("read reference keys: %w", err)
	}
	rbox, err := e.api.Body().RangeBox(wire.BodyRangeBoxArgs{BodyIndex: bodyIndex, Precise: true})
	if err != nil {
		return nil, Stock{}, fmt.Errorf("read range box of body %d: %w", bodyIndex, err)
	}
	holes, err := DetectDrillTargets(refs, rbox, bodyIndex)
	if err != nil {
		return nil, Stock{}, err
	}
	return holes, StockFromRangeBox(rbox.Min, rbox.Max), nil
}

// buildDrillingJob reads the body's topology + extent over the API and assembles a drilling
// job: stock from the range box, one drill tool controller, and a Drilling operation over
// the detected holes with depths framed to the stock. Returns the job and the detected
// holes (for the viewport overlay).
func (e *Engine) buildDrillingJob(bodyIndex int) (*Job, []DrillTarget, error) {
	holes, stock, err := e.detectHolesAndStock(bodyIndex)
	if err != nil {
		return nil, nil, err
	}

	job := NewJob()
	job.Stock = stock
	job.ModelBodies = []int{bodyIndex}
	job.Tools = e.jobTools()
	job.Operations = []Operation{&DrillingOp{
		OpBase: drillEnvelope(stock, indexForShape(job.Tools, "drill")),
		Holes:  holes,
	}}
	return job, holes, nil
}

// drillEnvelope builds the depth/height envelope for a drilling operation framed to the stock
// (rapid clearance + canned-cycle R plane above the top, drilling from the top through the
// bottom), on the given tool controller.
func drillEnvelope(stock Stock, toolController int) OpBase {
	return OpBase{
		OpLabel: "Drilling", IsActive: true, ToolController: toolController,
		ClearanceHeight: stock.TopZ() + drillClearanceAbove,
		SafeHeight:      stock.TopZ() + drillRetractAbove,
		RetractHeight:   stock.TopZ() + drillRetractAbove,
		StartDepth:      stock.TopZ(),
		FinalDepth:      stock.BottomZ(),
	}
}

// ToolpathOverlayID is the stable client-graphics id the CAM toolpath overlay owns.
const ToolpathOverlayID = "com.oblikovati.cam.toolpath"

// pushToolpathOverlay draws the drilling toolpath as green vertical segments (one per hole,
// top to bottom) in the viewport. Coordinates are converted from CAM millimetres back to
// the host's centimetre database unit. Best-effort: a graphics failure must not fail the
// job (the G-code is the deliverable).
func (e *Engine) pushToolpathOverlay(holes []DrillTarget) (string, error) {
	coords, indices := holeSegments(holes)
	if len(indices) == 0 {
		return "", nil
	}
	green := []float32{0.1, 0.9, 0.2, 1.0}
	if _, err := e.api.Graphics().AddLines(ToolpathOverlayID, coords, indices, green); err != nil {
		return "", err
	}
	return ToolpathOverlayID, nil
}

// holeSegments builds the indexed line list for the hole overlay: two vertices (top, bottom)
// per hole in centimetres, paired by consecutive indices.
func holeSegments(holes []DrillTarget) ([]float64, []int) {
	coords := make([]float64, 0, len(holes)*6)
	indices := make([]int, 0, len(holes)*2)
	for i, h := range holes {
		coords = append(coords,
			h.X/cmToMM, h.Y/cmToMM, h.Top/cmToMM,
			h.X/cmToMM, h.Y/cmToMM, h.Bottom/cmToMM,
		)
		indices = append(indices, 2*i, 2*i+1)
	}
	return coords, indices
}
