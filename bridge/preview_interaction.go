// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// PreviewNodeID is the id of the transient toolpath-preview node in the interaction overlay
// lane. Re-sending it replaces the previous preview.
const PreviewNodeID = "com.oblikovati.cam.preview"

// cutPreviewColor / rapidPreviewColor are the preview line colours (green cuts, grey rapids).
var (
	cutPreviewColor   = []float32{0.1, 0.9, 0.2, 1}
	rapidPreviewColor = []float32{0.6, 0.6, 0.6, 1}
)

// updateToolpathPreview draws the generated toolpath into the transient interaction overlay
// lane — a command-scoped, on-top preview the host replaces on every call and clears when the
// command ends. It is distinct from the persistent committed overlay (Graphics().AddLines):
// the preview is the live drag/manipulator feedback shown while a parameter is being adjusted,
// before the job is committed. Cuts draw green, rapids grey. An empty path clears the preview.
func (e *Engine) updateToolpathPreview(results []OperationResult) error {
	var rapids, cuts PreviewLines
	for _, r := range results {
		rr, cc := ToolpathPreview(r.Path)
		rapids, cuts = mergeLines(rapids, rr), mergeLines(cuts, cc)
	}
	var prims []wire.GraphicsPrimitive
	if len(cuts.Indices) > 0 {
		prims = append(prims, linePrimitive(cuts, cutPreviewColor))
	}
	if len(rapids.Indices) > 0 {
		prims = append(prims, linePrimitive(rapids, rapidPreviewColor))
	}
	if len(prims) == 0 {
		return e.clearToolpathPreview()
	}
	node := wire.GraphicsNode{Id: PreviewNodeID, Primitives: prims}
	return e.api.Graphics().Interaction().Update(types.GraphicsLaneOverlay, []wire.GraphicsNode{node})
}

// clearToolpathPreview removes the transient toolpath preview from the interaction lanes.
func (e *Engine) clearToolpathPreview() error {
	return e.api.Graphics().Interaction().Clear()
}

// clearPreviewAction is the Clear-Preview command handler.
func (e *Engine) clearPreviewAction() (*JobResult, error) {
	if err := e.clearToolpathPreview(); err != nil {
		return nil, err
	}
	return &JobResult{Summary: "CAM: toolpath preview cleared."}, nil
}

// linePrimitive wraps a preview line list as an indexed-lines graphics primitive in one colour.
func linePrimitive(lines PreviewLines, color []float32) wire.GraphicsPrimitive {
	return wire.GraphicsPrimitive{
		Kind: string(types.GraphicsLines), Coordinates: lines.Coords, Indices: lines.Indices, Color: color,
	}
}

// PreviewProfileOnHost generates the profile toolpath and shows it on the transient
// interaction lane WITHOUT posting G-code or committing the persistent overlay — the live
// preview a user sees while setting up the op. The job is remembered so a following Generate
// or Save acts on the previewed configuration.
func (e *Engine) PreviewProfileOnHost(bodyIndex int) (*JobResult, error) {
	job, _, err := e.buildProfileJob(bodyIndex)
	if err != nil {
		return nil, err
	}
	results, err := job.GenerateAll()
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	job.PostProcessor = e.postName
	e.lastJob = job
	e.mu.Unlock()
	if err := e.updateToolpathPreview(results); err != nil {
		return nil, err
	}
	return &JobResult{Summary: "CAM: previewing the profile toolpath (not yet committed)."}, nil
}
