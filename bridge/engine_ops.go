// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/gen"
	"oblikovati.org/cam/bridge/geom2d"
	"oblikovati.org/cam/bridge/post"
)

// Defaults for the milestone-3 demo run methods.
const (
	helixHoleRadius = 8.0  // mm — bored-hole radius for the helix demo
	helixPitch      = 1.5  // mm/turn
	threadMajorDia  = 10.0 // mm — nominal thread diameter for the thread-mill demo
	threadPitch     = 1.5  // mm — thread lead per turn
	cboreDiameter   = 12.0 // mm — counterbore recess diameter
	cboreDepth      = 4.0  // mm — counterbore recess depth
	cborePitch      = 1.0  // mm — counterbore helix pitch
	tapPitch        = 1.5  // mm — thread pitch for the tapping demo (e.g. M10×1.5)
	csinkDiameter   = 10.0 // mm — countersink rim diameter
	csinkAngle      = 90.0 // deg — countersink tool included angle
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

// probeApproachGap is how far (mm) outside a face the probe starts its approach, and
// probeOvertravel how far past the expected face the probe is allowed to travel before erroring.
const (
	probeApproachGap = 5.0
	probeOvertravel  = 5.0
)

// RunCounterboreJobOnHost spot-faces a flat-bottom recess at each of the part's hole tops.
func (e *Engine) RunCounterboreJobOnHost(bodyIndex int) (*JobResult, error) {
	holes, stock, err := e.detectHolesAndStock(bodyIndex)
	if err != nil {
		return nil, err
	}
	job := e.newMillJob(bodyIndex, stock)
	job.Operations = []Operation{&CounterboreOp{
		OpBase:   e.millEnvelope("Counterbore", stock),
		Diameter: cboreDiameter, Depth: cboreDepth, Pitch: cborePitch,
		Holes: holes,
	}}
	return e.postPreviewResult(job, fmt.Sprintf("counterbored %d hole(s)", len(holes)))
}

// RunTappingJobOnHost cuts internal threads in each of the part's holes with a synchronised tap
// cycle, feeding at the thread pitch per spindle revolution.
func (e *Engine) RunTappingJobOnHost(bodyIndex int) (*JobResult, error) {
	holes, stock, err := e.detectHolesAndStock(bodyIndex)
	if err != nil {
		return nil, err
	}
	job := e.newMillJob(bodyIndex, stock)
	job.Operations = []Operation{&TappingOp{
		OpBase: e.millEnvelope("Tapping", stock),
		Pitch:  tapPitch,
		Holes:  holes,
	}}
	return e.postPreviewResult(job, fmt.Sprintf("tapped %d hole(s)", len(holes)))
}

// RunCountersinkJobOnHost cuts a conical recess at each of the part's hole tops.
func (e *Engine) RunCountersinkJobOnHost(bodyIndex int) (*JobResult, error) {
	holes, stock, err := e.detectHolesAndStock(bodyIndex)
	if err != nil {
		return nil, err
	}
	job := e.newMillJob(bodyIndex, stock)
	job.Operations = []Operation{&CountersinkOp{
		OpBase:   e.millEnvelope("Countersink", stock),
		Diameter: csinkDiameter, ToolAngle: csinkAngle,
		Holes: holes,
	}}
	return e.postPreviewResult(job, fmt.Sprintf("countersank %d hole(s)", len(holes)))
}

// boreProbeReach is how far (mm) a bore probe travels outward from the hole centre before
// erroring if it never reaches a wall.
const boreProbeReach = 25.0

// RunBoreProbeJobOnHost finds each detected hole's centre by probing its wall in four directions.
func (e *Engine) RunBoreProbeJobOnHost(bodyIndex int) (*JobResult, error) {
	holes, stock, err := e.detectHolesAndStock(bodyIndex)
	if err != nil {
		return nil, err
	}
	if len(holes) == 0 {
		return nil, fmt.Errorf("bore probing found no holes to centre on body %d", bodyIndex)
	}
	job := e.newMillJob(bodyIndex, stock)
	job.Operations = []Operation{&ProbeOp{
		OpBase: e.millEnvelope("Bore Probe", stock),
		Points: boreProbePoints(holes),
	}}
	return e.postPreviewResult(job, fmt.Sprintf("bore-probed %d hole(s)", len(holes)))
}

// boreProbePoints builds a four-touch centre cycle for each hole: from the hole centre at mid
// depth, probe outward to the wall toward +X, −X, +Y, and −Y (the wall trip points bisect to the
// true centre). The probe retracts to clearance and re-approaches the centre between touches.
func boreProbePoints(holes []DrillTarget) []gen.ProbePoint {
	var pts []gen.ProbePoint
	for _, h := range orderedHoles(holes) {
		midZ := (h.Top + h.Bottom) / 2
		centre := gcode.Vector3{X: h.X, Y: h.Y, Z: midZ}
		for _, d := range [][2]float64{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			pts = append(pts, gen.ProbePoint{
				Approach: centre,
				Target:   gcode.Vector3{X: h.X + d[0]*boreProbeReach, Y: h.Y + d[1]*boreProbeReach, Z: midZ},
			})
		}
	}
	return pts
}

// Default tool-setter position (machine coordinates, mm) the engine seeds a tool-length probe
// with; the operator adjusts these to their machine in the operation editor.
const (
	toolSetterX   = -50.0
	toolSetterY   = -50.0
	toolSetterTop = 25.0
)

// RunToolLengthProbeJobOnHost measures the active tool against the tool-setter and sets its
// length offset.
func (e *Engine) RunToolLengthProbeJobOnHost(bodyIndex int) (*JobResult, error) {
	_, stock, err := e.detectHolesAndStock(bodyIndex)
	if err != nil {
		return nil, err
	}
	job := e.newMillJob(bodyIndex, stock)
	job.Operations = []Operation{&ToolLengthProbeOp{
		OpBase:  e.millEnvelope("Tool Probe", stock),
		SetterX: toolSetterX, SetterY: toolSetterY, SetterTop: toolSetterTop, ToolNumber: 1,
	}}
	return e.postPreviewResult(job, "measured the tool length against the setter")
}

// RunBossProbeJobOnHost finds the part footprint's centre by probing its outline walls inward
// from four sides — the outward-in counterpart of bore probing.
func (e *Engine) RunBossProbeJobOnHost(bodyIndex int) (*JobResult, error) {
	boundary, stock, err := e.contourAndStock(bodyIndex)
	if err != nil {
		return nil, err
	}
	job := e.newMillJob(bodyIndex, stock)
	job.Operations = []Operation{&ProbeOp{
		OpBase: e.millEnvelope("Boss Probe", stock),
		Points: bossProbePoints(boundary, stock),
	}}
	return e.postPreviewResult(job, "boss-probed the part outline")
}

// bossProbePoints builds a four-touch centre cycle on a raised feature: from just outside the
// outline's bounding box on each side, probe inward toward the centre until the wall trips (the
// four trip points bisect to the footprint centre). No axis is zeroed — the centre comes from
// averaging, like bore probing.
func bossProbePoints(boundary geom2d.Polygon, stock Stock) []gen.ProbePoint {
	minX, minY, maxX, maxY := boundaryBounds(boundary)
	cx, cy := (minX+maxX)/2, (minY+maxY)/2
	midZ := (stock.Min.Z + stock.Max.Z) / 2
	return []gen.ProbePoint{
		{Approach: gcode.Vector3{X: maxX + probeApproachGap, Y: cy, Z: midZ}, Target: gcode.Vector3{X: cx, Y: cy, Z: midZ}}, // +X wall, probe −X
		{Approach: gcode.Vector3{X: minX - probeApproachGap, Y: cy, Z: midZ}, Target: gcode.Vector3{X: cx, Y: cy, Z: midZ}}, // −X wall, probe +X
		{Approach: gcode.Vector3{X: cx, Y: maxY + probeApproachGap, Z: midZ}, Target: gcode.Vector3{X: cx, Y: cy, Z: midZ}}, // +Y wall, probe −Y
		{Approach: gcode.Vector3{X: cx, Y: minY - probeApproachGap, Z: midZ}, Target: gcode.Vector3{X: cx, Y: cy, Z: midZ}}, // −Y wall, probe +Y
	}
}

// boundaryBounds returns the axis-aligned bounding box of a polygon (mm).
func boundaryBounds(poly geom2d.Polygon) (minX, minY, maxX, maxY float64) {
	minX, minY = poly[0].X, poly[0].Y
	maxX, maxY = poly[0].X, poly[0].Y
	for _, p := range poly[1:] {
		minX, minY = minF(minX, p.X), minF(minY, p.Y)
		maxX, maxY = maxF(maxX, p.X), maxF(maxY, p.Y)
	}
	return minX, minY, maxX, maxY
}

// minF / maxF are float min/max helpers (kept local to avoid a math import here).
func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// RunProbeJobOnHost probes the stock's top and two edges to find the work origin.
func (e *Engine) RunProbeJobOnHost(bodyIndex int) (*JobResult, error) {
	_, stock, err := e.detectHolesAndStock(bodyIndex)
	if err != nil {
		return nil, err
	}
	job := e.newMillJob(bodyIndex, stock)
	job.Operations = []Operation{&ProbeOp{
		OpBase: e.millEnvelope("Probe", stock),
		Points: cornerProbePoints(stock),
	}}
	return e.postPreviewResult(job, "probed the stock top and two edges")
}

// cornerProbePoints builds a three-touch corner cycle on the stock's minimum corner: a Z
// touch-off on the top, then an edge probe toward the −X and −Y faces at mid height.
func cornerProbePoints(stock Stock) []gen.ProbePoint {
	midZ := (stock.Min.Z + stock.Max.Z) / 2
	inX, inY := stock.Min.X+probeApproachGap*2, stock.Min.Y+probeApproachGap*2
	return []gen.ProbePoint{
		{ // top surface: drop onto it, zero Z there
			Approach: gcode.Vector3{X: inX, Y: inY, Z: stock.Max.Z + probeApproachGap},
			Target:   gcode.Vector3{X: inX, Y: inY, Z: stock.Min.Z},
			SetAxis:  "Z",
		},
		{ // −X face: probe toward +X, zero X there
			Approach: gcode.Vector3{X: stock.Min.X - probeApproachGap, Y: inY, Z: midZ},
			Target:   gcode.Vector3{X: stock.Min.X + probeOvertravel, Y: inY, Z: midZ},
			SetAxis:  "X",
		},
		{ // −Y face: probe toward +Y, zero Y there
			Approach: gcode.Vector3{X: inX, Y: stock.Min.Y - probeApproachGap, Z: midZ},
			Target:   gcode.Vector3{X: inX, Y: stock.Min.Y + probeOvertravel, Z: midZ},
			SetAxis:  "Y",
		},
	}
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

// vcarveAngle is the default V-bit included angle (degrees) for the V-carve demo.
const vcarveAngle = 90.0

// RunVCarveJobOnHost V-carves the part's outline region with a V-bit.
func (e *Engine) RunVCarveJobOnHost(bodyIndex int) (*JobResult, error) {
	boundary, stock, err := e.contourAndStock(bodyIndex)
	if err != nil {
		return nil, err
	}
	job := e.newMillJob(bodyIndex, stock)
	env := e.millEnvelope("V-Carve", stock)
	env.StartDepth = stock.TopZ()
	job.Operations = []Operation{&VCarveOp{OpBase: env, ToolAngle: vcarveAngle, Boundary: boundary}}
	return e.postPreviewResult(job, "v-carved the outline region")
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
	gcodeText, err := post.Export(postName, PostObjects(results), e.postArgs())
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
