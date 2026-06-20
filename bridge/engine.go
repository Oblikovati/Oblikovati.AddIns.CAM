// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"encoding/json"
	"fmt"
	"sync"

	"oblikovati.org/api/client"
	"oblikovati.org/api/wire"
	"oblikovati.org/cam/bridge/post"
)

// HostCaller is the transport the engine talks to the host through — exactly the
// api/client Caller contract, supplied by the cgo shell at Activate (or a fake in tests).
// Keeping it an interface here keeps this package cgo-free and testable.
type HostCaller interface {
	Call(method string, req []byte) ([]byte, error)
}

// Engine runs CAM jobs against a live host: it reads the part's geometry over the API,
// generates toolpaths, posts G-code, and renders a toolpath overlay back into the viewport.
type Engine struct {
	host HostCaller
	api  *client.Client

	mu        sync.Mutex
	running   bool   // a job is in flight (coalesces overlapping command triggers)
	postName  string // active post processor ("linuxcnc" | "grbl")
	plungFeed float64
	lastJob   *Job // most recently generated job (for the operations browser + Save)
}

// NewEngine binds the engine to the host transport with milestone-1 defaults.
func NewEngine(host HostCaller) *Engine {
	return &Engine{host: host, api: client.New(host), postName: "linuxcnc", plungFeed: defaultPlungeFeed}
}

// defaultPlungeFeed is the default drilling plunge feed (mm/min) until the panel overrides it.
const defaultPlungeFeed = 100.0

// SetPost selects the post processor ("linuxcnc" | "grbl"); an empty/unknown name leaves the
// current one. Returns the engine for chaining. Stand-in for the panel's post dropdown.
func (e *Engine) SetPost(name string) *Engine {
	if name == "linuxcnc" || name == "grbl" {
		e.mu.Lock()
		e.postName = name
		e.mu.Unlock()
	}
	return e
}

// The CAM commands the add-in registers; firing one (a ribbon click or the MCP bridge's
// execute_command) generates the corresponding toolpath for the active part.
const (
	GenerateJobCommandID      = "CAM.GenerateJob"      // drilling (kept stable for M1 callers)
	GenerateProfileCommandID  = "CAM.GenerateProfile"  // contour profile
	GeneratePocketCommandID   = "CAM.GeneratePocket"   // area-clearing pocket
	GenerateHelixCommandID    = "CAM.GenerateHelix"    // helical bore
	GenerateMillFaceCommandID = "CAM.GenerateMillFace" // face milling
	GenerateEngraveCommandID  = "CAM.GenerateEngrave"  // engraving
	ShowOperationsCommandID   = "CAM.ShowOperations"   // open the operations browser
	SaveJobCommandID          = "CAM.SaveJob"          // persist the job into the document
	LoadJobCommandID          = "CAM.LoadJob"          // load the job from the document
)

// camCommands describes each registered command for registration + the panel.
var camCommands = []struct{ id, name, tip string }{
	{GenerateJobCommandID, "Generate Drilling Job", "Detect the part's holes, generate a drilling toolpath, and post it to G-code."},
	{GenerateProfileCommandID, "Generate Profile Job", "Contour the part's outline with tool compensation, and post it to G-code."},
	{GeneratePocketCommandID, "Generate Pocket Job", "Clear the part's outline region with concentric passes, and post it to G-code."},
	{GenerateHelixCommandID, "Generate Helix Job", "Bore the part's holes with a helix (for holes wider than the tool)."},
	{GenerateMillFaceCommandID, "Generate Face Job", "Face the top of the stock over the part's outline."},
	{GenerateEngraveCommandID, "Generate Engrave Job", "Engrave the part's outline on the tool centre."},
	{ShowOperationsCommandID, "Show Operations", "Open the CAM operations browser for the last generated job."},
	{SaveJobCommandID, "Save CAM Job", "Persist the CAM job into the active document."},
	{LoadJobCommandID, "Load CAM Job", "Load the CAM job stored in the active document."},
}

// RegisterCommands registers the CAM commands so each is invokable like a ribbon click. The
// host action is a no-op; executing one fires command.started, which Notify turns into a job.
func (e *Engine) RegisterCommands() error {
	for _, c := range camCommands {
		if _, err := e.api.Commands().Create(wire.CreateCommandArgs{
			ID: c.id, DisplayName: c.name, Category: "CAM", Tooltip: c.tip,
		}); err != nil {
			return err
		}
	}
	return nil
}

// Setup performs the one-time host-facing initialisation: register the command and show the
// CAM panel. It MUST NOT run on the host's session goroutine (host calls there deadlock the
// head) — the cgo shell runs it on its own goroutine.
func (e *Engine) Setup() error {
	if err := e.RegisterCommands(); err != nil {
		return err
	}
	_, err := e.ShowPanel()
	return err
}

// Notify receives host event bytes. A command.started carrying GenerateJobCommandID runs a
// job on a SEPARATE goroutine — never inline, because Notify runs on the host's session
// goroutine and a host call from there deadlocks. A panel edit only mutates engine state.
func (e *Engine) Notify(ev []byte) {
	var hdr struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(ev, &hdr) != nil {
		return
	}
	switch hdr.Type {
	case wire.EventCommandStarted:
		var c struct {
			Command string `json:"command"`
		}
		if json.Unmarshal(ev, &c) == nil {
			e.dispatchCommand(c.Command)
		}
	case wire.EventPanelValueChanged:
		var p struct {
			WindowId  string `json:"windowId"`
			ControlId string `json:"controlId"`
			Value     string `json:"value"`
		}
		if json.Unmarshal(ev, &p) == nil && p.WindowId == CAMPanelID {
			e.applyPanelEdit(p.ControlId, p.Value)
		}
	}
}

// dispatchCommand maps a fired command id to the job that produces its toolpath, ignoring
// commands the add-in does not own.
func (e *Engine) dispatchCommand(commandID string) {
	switch commandID {
	case GenerateJobCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunDrillingJobOnHost(0) })
	case GenerateProfileCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunProfileJobOnHost(0) })
	case GeneratePocketCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunPocketJobOnHost(0) })
	case GenerateHelixCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunHelixJobOnHost(0) })
	case GenerateMillFaceCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunMillFaceJobOnHost(0) })
	case GenerateEngraveCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunEngraveJobOnHost(0) })
	case ShowOperationsCommandID:
		e.launchRun(e.showOperationsAction)
	case SaveJobCommandID:
		e.launchRun(e.saveJobAction)
	case LoadJobCommandID:
		e.launchRun(e.loadJobAction)
	}
}

// launchRun runs one job goroutine, coalescing overlapping triggers, and reports the outcome
// to the host status bar. The job MUST run off the session goroutine (Notify's caller),
// because a host call there would deadlock.
func (e *Engine) launchRun(run func() (*JobResult, error)) {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.mu.Unlock()

	go func() {
		defer func() {
			e.mu.Lock()
			e.running = false
			e.mu.Unlock()
		}()
		res, err := run()
		if err != nil {
			e.reportStatus("CAM job failed: " + err.Error())
			return
		}
		e.reportStatus(res.Summary)
	}()
}

// reportStatus surfaces a job's outcome on the host status bar (best-effort).
func (e *Engine) reportStatus(msg string) { _, _ = e.api.Status().SetText(msg) }

// JobResult summarizes one generated job.
type JobResult struct {
	GCode      string
	HoleCount  int // drilling only
	GCodeLines int
	OverlayID  string
	Summary    string // human status line
}

// RunDrillingJobOnHost is the end-to-end add-in flow for one body: read its topology and
// extent over the API, build the drilling job, post it to G-code, and push the toolpath
// overlay into the viewport. Returns the G-code and a summary.
func (e *Engine) RunDrillingJobOnHost(bodyIndex int) (*JobResult, error) {
	job, holes, err := e.buildDrillingJob(bodyIndex)
	if err != nil {
		return nil, err
	}
	gcodeText, err := e.GenerateGCode(job)
	if err != nil {
		return nil, err
	}
	overlayID, _ := e.pushToolpathOverlay(holes) // best-effort viewport preview
	lines := countLines(gcodeText)
	return &JobResult{
		GCode:      gcodeText,
		HoleCount:  len(holes),
		GCodeLines: lines,
		OverlayID:  overlayID,
		Summary:    fmt.Sprintf("CAM: drilled %d hole(s), %d G-code lines (%s).", len(holes), lines, e.postName),
	}, nil
}

// GenerateGCode runs every active operation and posts the result with the engine's active
// post processor.
func (e *Engine) GenerateGCode(job *Job) (string, error) {
	results, err := job.GenerateAll()
	if err != nil {
		return "", err
	}
	e.mu.Lock()
	job.PostProcessor = e.postName
	e.lastJob = job
	e.mu.Unlock()
	return post.Export(job.PostProcessor, PostObjects(results), "--no-show-editor")
}

// countLines counts the newline-terminated lines in the G-code.
func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := 0
	for _, r := range s {
		if r == '\n' {
			n++
		}
	}
	return n
}
