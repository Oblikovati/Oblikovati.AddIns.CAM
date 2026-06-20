// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/api/wire"
	"oblikovati.org/cam/bridge/geom2d"
)

// millStepDown is the default Z step per pass (mm) for milling operations, and millEndmillDia
// the default end-mill diameter (mm) until the panel/job overrides them.
const (
	millStepDown   = 3.0
	millEndmillDia = 6.0
)

// RunProfileJobOnHost contours the body's outline: extract the silhouette, build a profile
// job, post it, and overlay the toolpath. Returns the G-code and a summary.
func (e *Engine) RunProfileJobOnHost(bodyIndex int) (*JobResult, error) {
	job, boundary, err := e.buildProfileJob(bodyIndex)
	if err != nil {
		return nil, err
	}
	return e.finishMillJob(job, boundary, "profiled")
}

// RunPocketJobOnHost clears the body's outline region: extract the silhouette, build a pocket
// job, post it, and overlay the toolpath.
func (e *Engine) RunPocketJobOnHost(bodyIndex int) (*JobResult, error) {
	job, boundary, err := e.buildPocketJob(bodyIndex)
	if err != nil {
		return nil, err
	}
	return e.finishMillJob(job, boundary, "pocketed")
}

// finishMillJob posts a milling job, overlays its boundary contour, and builds the summary.
func (e *Engine) finishMillJob(job *Job, boundary geom2d.Polygon, verb string) (*JobResult, error) {
	gcodeText, err := e.GenerateGCode(job)
	if err != nil {
		return nil, err
	}
	overlayID, _ := e.pushContourOverlay(boundary, job.Stock.TopZ())
	_ = e.clearToolpathPreview() // the committed overlay replaces any transient preview
	lines := countLines(gcodeText)
	e.mu.Lock()
	source := e.sectionSource
	e.mu.Unlock()
	return &JobResult{
		GCode:      gcodeText,
		GCodeLines: lines,
		OverlayID:  overlayID,
		Summary:    fmt.Sprintf("CAM: %s the outline from the %s (%d boundary pts), %d G-code lines (%s).", verb, source, len(boundary), lines, e.postName),
	}, nil
}

// buildProfileJob assembles an outside-contour profile job over the body's silhouette.
func (e *Engine) buildProfileJob(bodyIndex int) (*Job, geom2d.Polygon, error) {
	boundary, stock, err := e.contourAndStock(bodyIndex)
	if err != nil {
		return nil, nil, err
	}
	cut := e.cutting()
	job := e.newMillJob(bodyIndex, stock)
	job.Operations = []Operation{&ProfileOp{
		OpBase:   e.millEnvelope("Profile", stock),
		Side:     "outside",
		Climb:    true,
		StepDown: cut.StepDown,
		Boundary: boundary,
	}}
	return job, boundary, nil
}

// buildPocketJob assembles an area-clearing pocket job over the body's silhouette region.
func (e *Engine) buildPocketJob(bodyIndex int) (*Job, geom2d.Polygon, error) {
	boundary, stock, err := e.contourAndStock(bodyIndex)
	if err != nil {
		return nil, nil, err
	}
	cut := e.cutting()
	job := e.newMillJob(bodyIndex, stock)
	job.Operations = []Operation{&PocketOp{
		OpBase:   e.millEnvelope("Pocket", stock),
		StepOver: cut.StepOver,
		Climb:    true,
		StepDown: cut.StepDown,
		Boundary: boundary,
	}}
	return job, boundary, nil
}

// newMillJob builds a job with stock and the loaded tool controllers (the primary end mill plus
// the library tools). Milling operations run on the end mill at index 0; the 3D ops select the
// ball-nose. Mirrors a FreeCAD job loading its tool controllers.
func (e *Engine) newMillJob(bodyIndex int, stock Stock) *Job {
	job := NewJob()
	job.Stock = stock
	job.ModelBodies = []int{bodyIndex}
	job.Tools = e.jobTools()
	return job
}

// millEnvelope builds the depth/height envelope for a milling op framed to the stock:
// clearance/safe planes above the top, cutting from the top down to the user's cut depth (or
// through to the stock bottom when the cut depth is zero).
func (e *Engine) millEnvelope(label string, stock Stock) OpBase {
	cut := e.cutting()
	return OpBase{
		OpLabel: label, IsActive: true, ToolController: 0,
		ClearanceHeight: cut.clearanceZ(stock.TopZ()),
		SafeHeight:      cut.retractZ(stock.TopZ()),
		RetractHeight:   cut.retractZ(stock.TopZ()),
		StartDepth:      stock.TopZ(),
		FinalDepth:      cut.finalDepth(stock.TopZ(), stock.BottomZ()),
	}
}

// contourAndStock reads the body's extent and sections it to obtain the outer silhouette
// contour (the largest section wire), returned in millimetres along with the stock. The
// section plane follows a selected planar face when one is picked, otherwise a horizontal
// plane at the body's mid-height — both give a clean outline for a prismatic part.
func (e *Engine) contourAndStock(bodyIndex int) (geom2d.Polygon, Stock, error) {
	rbox, err := e.api.Body().RangeBox(wire.BodyRangeBoxArgs{BodyIndex: bodyIndex, Precise: true})
	if err != nil {
		return nil, Stock{}, fmt.Errorf("read range box of body %d: %w", bodyIndex, err)
	}
	if len(rbox.Min) < 3 || len(rbox.Max) < 3 {
		return nil, Stock{}, fmt.Errorf("body %d has no extent", bodyIndex)
	}
	stock := e.stockFor(rbox.Min, rbox.Max)
	plane, err := e.sectionPlaneFor(bodyIndex, rbox)
	if err != nil {
		return nil, Stock{}, err
	}
	e.mu.Lock()
	e.sectionSource = plane.source
	e.mu.Unlock()

	bi := bodyIndex
	section, err := e.api.TransientBRep().CreateIntersectionWithPlane(
		wire.BrepBodyRef{BodyIndex: &bi}, plane.origin, plane.normal)
	if err != nil {
		return nil, Stock{}, fmt.Errorf("section body %d at %s plane: %w", bodyIndex, plane.source, err)
	}
	boundary := largestContour(section.Wires)
	if len(boundary) < 3 {
		return nil, Stock{}, fmt.Errorf("section of body %d produced no usable outline (got %d wires)", bodyIndex, len(section.Wires))
	}
	return boundary, stock, nil
}

// largestContour converts the section wires (flat xyz in cm) to XY polygons in millimetres
// and returns the one enclosing the greatest area — the outer boundary (inner wires are
// holes, handled in a later milestone).
func largestContour(wires []wire.WirePolyline) geom2d.Polygon {
	var best geom2d.Polygon
	var bestArea float64
	for _, w := range wires {
		poly := wirePolyline(w)
		if a := poly.Area(); a > bestArea {
			best, bestArea = poly, a
		}
	}
	return best
}

// wirePolyline converts one sampled section wire (flat xyz triplets in cm) to an XY polygon
// in millimetres, dropping Z and the duplicated closing point.
func wirePolyline(w wire.WirePolyline) geom2d.Polygon {
	var poly geom2d.Polygon
	for i := 0; i+2 < len(w.Points); i += 3 {
		poly = append(poly, geom2d.Point2{X: w.Points[i] * cmToMM, Y: w.Points[i+1] * cmToMM})
	}
	if n := len(poly); n > 1 && poly[n-1] == poly[0] {
		poly = poly[:n-1]
	}
	return poly
}

// pushContourOverlay draws the boundary contour as a closed green loop at height z (mm) in
// the viewport, converting back to centimetres. Best-effort.
func (e *Engine) pushContourOverlay(boundary geom2d.Polygon, z float64) (string, error) {
	if len(boundary) < 2 {
		return "", nil
	}
	coords := make([]float64, 0, len(boundary)*3)
	for _, p := range boundary {
		coords = append(coords, p.X/cmToMM, p.Y/cmToMM, z/cmToMM)
	}
	indices := make([]int, 0, len(boundary)*2)
	for i := range boundary {
		indices = append(indices, i, (i+1)%len(boundary)) // close the loop
	}
	green := []float32{0.1, 0.9, 0.2, 1.0}
	if _, err := e.api.Graphics().AddLines(ToolpathOverlayID, coords, indices, green); err != nil {
		return "", err
	}
	return ToolpathOverlayID, nil
}
