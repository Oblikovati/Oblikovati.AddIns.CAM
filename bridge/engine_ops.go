// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gen"
	"oblikovati.org/cam/bridge/post"
)

// Defaults for the milestone-3 demo run methods.
const (
	helixHoleRadius = 8.0  // mm — bored-hole radius for the helix demo
	helixPitch      = 1.5  // mm/turn
	threadMajorDia  = 10.0 // mm — nominal thread diameter for the thread-mill demo
	threadPitch     = 1.5  // mm — thread lead per turn
	faceDepth       = 1.0  // mm — facing depth off the stock top
	engraveDepth    = 0.5  // mm — engraving depth off the stock top
	chamferWidth    = 1.0  // mm — chamfer/deburr bevel width
	chamferAngle    = 90.0 // deg — chamfer V-tool included angle
)

// RapidOverlayID is the client-graphics id for the rapid-move (grey) overlay, drawn beside
// the cutting (green) overlay so rapids read distinctly.
const RapidOverlayID = "com.oblikovati.cam.rapids"

// RunHelixJobOnHost bores the part's holes with a helix (for holes wider than the tool).
func (e *Engine) RunHelixJobOnHost(bodyIndex int) (*JobResult, error) {
	holes, stock, err := e.detectHolesAndStock(bodyIndex)
	if err != nil {
		return nil, err
	}
	job := e.newMillJob(bodyIndex, stock)
	job.Operations = []Operation{&HelixOp{
		OpBase:     e.millEnvelope("Helix", stock),
		HoleRadius: helixHoleRadius, Pitch: helixPitch, Direction: gen.HelixCW,
		Holes: holes,
	}}
	return e.postPreviewResult(job, fmt.Sprintf("bored %d hole(s) by helix", len(holes)))
}

// RunThreadMillJobOnHost cuts a thread into each of the part's holes by helical interpolation.
func (e *Engine) RunThreadMillJobOnHost(bodyIndex int) (*JobResult, error) {
	holes, stock, err := e.detectHolesAndStock(bodyIndex)
	if err != nil {
		return nil, err
	}
	job := e.newMillJob(bodyIndex, stock)
	job.Operations = []Operation{&ThreadMillOp{
		OpBase:        e.millEnvelope("Thread", stock),
		MajorDiameter: threadMajorDia, Pitch: threadPitch, Internal: true, Climb: true,
		Holes: holes,
	}}
	return e.postPreviewResult(job, fmt.Sprintf("thread-milled %d hole(s)", len(holes)))
}

// RunMillFaceJobOnHost faces the top of the stock over the part's outline.
func (e *Engine) RunMillFaceJobOnHost(bodyIndex int) (*JobResult, error) {
	boundary, stock, err := e.contourAndStock(bodyIndex)
	if err != nil {
		return nil, err
	}
	cut := e.cutting()
	job := e.newMillJob(bodyIndex, stock)
	env := e.millEnvelope("Face", stock)
	env.FinalDepth = stock.TopZ() - faceDepth
	job.Operations = []Operation{&MillFaceOp{OpBase: env, StepOver: cut.StepOver, StepDown: cut.StepDown, Boundary: boundary}}
	return e.postPreviewResult(job, "faced the top")
}

// RunEngraveJobOnHost engraves the part's outline on the tool centre.
func (e *Engine) RunEngraveJobOnHost(bodyIndex int) (*JobResult, error) {
	boundary, stock, err := e.contourAndStock(bodyIndex)
	if err != nil {
		return nil, err
	}
	job := e.newMillJob(bodyIndex, stock)
	env := e.millEnvelope("Engrave", stock)
	env.FinalDepth = stock.TopZ() - engraveDepth
	job.Operations = []Operation{&EngraveOp{OpBase: env, Climb: true, Boundary: boundary}}
	return e.postPreviewResult(job, "engraved the outline")
}

// RunChamferJobOnHost breaks (bevels) the part's top edge with a V-tool chamfer pass.
func (e *Engine) RunChamferJobOnHost(bodyIndex int) (*JobResult, error) {
	boundary, stock, err := e.contourAndStock(bodyIndex)
	if err != nil {
		return nil, err
	}
	job := e.newMillJob(bodyIndex, stock)
	env := e.millEnvelope("Chamfer", stock)
	env.StartDepth = stock.TopZ()
	job.Operations = []Operation{&ChamferOp{
		OpBase: env, Width: chamferWidth, ToolAngle: chamferAngle, Side: gen.SideOutside, Climb: true, Boundary: boundary,
	}}
	return e.postPreviewResult(job, "chamfered the top edge")
}

// postPreviewResult runs the job, remembers it (for the operations browser + Save), pushes a
// two-colour toolpath preview (cuts green, rapids grey), and posts the G-code.
func (e *Engine) postPreviewResult(job *Job, verb string) (*JobResult, error) {
	results, err := job.GenerateAll()
	if err != nil {
		return nil, err
	}
	estimate := EstimateMinutes(results)
	e.mu.Lock()
	job.PostProcessor = e.postName
	e.lastJob = job
	e.lastEstimate = estimate
	postName := e.postName
	e.mu.Unlock()

	overlayID, _ := e.pushToolpathPreview(results)
	_ = e.clearToolpathPreview() // the committed overlay replaces any transient preview
	gcodeText, err := post.Export(postName, PostObjects(results), "--no-show-editor")
	if err != nil {
		return nil, err
	}
	e.rememberGCode(gcodeText)
	lines := countLines(gcodeText)
	return &JobResult{
		GCode: gcodeText, GCodeLines: lines, OverlayID: overlayID, EstimatedMinutes: estimate,
		Summary: withEstimate(fmt.Sprintf("CAM: %s, %d G-code lines (%s).", verb, lines, postName), estimate),
	}, nil
}

// pushToolpathPreview draws the generated toolpath as two overlays: cutting moves in green
// and rapids in grey, so the path reads at a glance. Best-effort.
func (e *Engine) pushToolpathPreview(results []OperationResult) (string, error) {
	var rapids, cuts PreviewLines
	for _, r := range results {
		rr, cc := ToolpathPreview(r.Path)
		rapids, cuts = mergeLines(rapids, rr), mergeLines(cuts, cc)
	}
	if len(cuts.Indices) > 0 {
		if _, err := e.api.Graphics().AddLines(ToolpathOverlayID, cuts.Coords, cuts.Indices, []float32{0.1, 0.9, 0.2, 1}); err != nil {
			return "", err
		}
	}
	if len(rapids.Indices) > 0 {
		_, _ = e.api.Graphics().AddLines(RapidOverlayID, rapids.Coords, rapids.Indices, []float32{0.6, 0.6, 0.6, 1})
	}
	return ToolpathOverlayID, nil
}

// mergeLines concatenates two line lists, re-basing the second's indices onto the first.
func mergeLines(a, b PreviewLines) PreviewLines {
	base := len(a.Coords) / 3
	a.Coords = append(a.Coords, b.Coords...)
	for _, idx := range b.Indices {
		a.Indices = append(a.Indices, idx+base)
	}
	return a
}
