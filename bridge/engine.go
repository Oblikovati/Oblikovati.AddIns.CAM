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

	mu            sync.Mutex
	running       bool    // a job is in flight (coalesces overlapping command triggers)
	postName      string  // active post processor ("linuxcnc" | "grbl" | "fanuc")
	plungFeed     float64 // plunge (vertical) feed; the cutting feed is 3× this
	spindleSpeed  float64 // active spindle speed (rev/min), set by the feeds & speeds calculator
	material      string  // selected workpiece material (drives the feeds & speeds calculator)
	flutes        int     // flute count of the primary end mill (drives the feeds & speeds feed)
	lastJob       *Job    // most recently generated job (for the operations browser + Save)
	lastGCode     string  // most recently posted G-code (for export to a file)
	lastEstimate  float64 // estimated cycle time (minutes) of the last posted job
	targetBody    int     // index of the body the generate commands machine
	editingOp     int     // index of the operation shown in the operation editor
	sectionSource string  // how the last contour plane was chosen ("selected face" | "mid-height")
	cut           cutSettings
	library       ToolLibrary // tools beyond the primary milling end mill (drill, ball-nose, …)
	surfacer      Surfacer
}

// NewEngine binds the engine to the host transport with milestone-1 defaults.
func NewEngine(host HostCaller) *Engine {
	return &Engine{host: host, api: client.New(host), postName: "linuxcnc", plungFeed: defaultPlungeFeed, spindleSpeed: defaultSpindleSpeed, material: defaultMaterial, flutes: feedsFluteCount, cut: defaultCutSettings(), library: DefaultToolLibrary(), surfacer: oclSurfacer{}}
}

// activeEndmill is the primary milling tool (T1), built from the panel's tool-diameter and feed
// fields. It is always present at index 0 of a job's tool list.
func (e *Engine) activeEndmill() ToolController {
	e.mu.Lock()
	feed, dia, rpm, flutes := e.plungFeed, e.cut.ToolDiameter, e.spindleSpeed, e.flutes
	e.mu.Unlock()
	return ToolController{
		Label: "End mill", ToolNumber: 1, SpindleSpeed: rpm, SpindleDir: "Forward",
		VertFeed: feed, HorizFeed: feed * 3, Tool: ToolBit{Name: "End mill", ShapeType: "endmill", Diameter: dia, Flutes: flutes},
	}
}

// jobTools returns the controllers loaded into a job: the primary end mill (T1) followed by the
// library tools. Operations select among them by cutter shape (indexForShape).
func (e *Engine) jobTools() []ToolController {
	e.mu.Lock()
	lib := e.library.snapshot()
	e.mu.Unlock()
	return append([]ToolController{e.activeEndmill()}, lib...)
}

// defaultPlungeFeed is the default drilling plunge feed (mm/min) until the panel overrides it,
// and defaultSpindleSpeed the default spindle speed (rev/min) until a material is chosen.
const (
	defaultPlungeFeed   = 100.0
	defaultSpindleSpeed = 5000.0
	feedsFluteCount     = 2           // default end-mill flute count until the panel changes it
	defaultMaterial     = "aluminium" // selected workpiece material until the panel changes it
)

// SetPost selects the post processor ("linuxcnc" | "grbl" | "fanuc" | "marlin" | "haas"); an empty/unknown
// name leaves the current one. Returns the engine for chaining. Stand-in for the panel's post dropdown.
func (e *Engine) SetPost(name string) *Engine {
	if name == "linuxcnc" || name == "grbl" || name == "fanuc" || name == "marlin" || name == "haas" || name == "heidenhain" {
		e.mu.Lock()
		e.postName = name
		e.mu.Unlock()
	}
	return e
}

// The CAM commands the add-in registers; firing one (a ribbon click or the MCP bridge's
// execute_command) generates the corresponding toolpath for the active part.
const (
	GenerateJobCommandID         = "CAM.GenerateJob"         // drilling (kept stable for M1 callers)
	GenerateProfileCommandID     = "CAM.GenerateProfile"     // contour profile
	GeneratePocketCommandID      = "CAM.GeneratePocket"      // area-clearing pocket
	GenerateAdaptiveCommandID    = "CAM.GenerateAdaptive"    // high-speed adaptive clearing
	GenerateRestCommandID        = "CAM.GenerateRest"        // rest machining (wall band a larger tool missed)
	GenerateTrochoidalCommandID  = "CAM.GenerateTrochoidal"  // trochoidal contour milling
	GenerateSlotCommandID        = "CAM.GenerateSlot"        // slot / groove milling
	GenerateProbeCommandID       = "CAM.GenerateProbe"       // workpiece touch probing
	GenerateBoreProbeCommandID   = "CAM.GenerateBoreProbe"   // bore-centre probing
	GenerateBossProbeCommandID   = "CAM.GenerateBossProbe"   // boss-centre probing
	GenerateToolProbeCommandID   = "CAM.GenerateToolProbe"   // tool-length probing
	GenerateHelixCommandID       = "CAM.GenerateHelix"       // helical bore
	GenerateThreadMillCommandID  = "CAM.GenerateThreadMill"  // thread milling
	GenerateCounterboreCommandID = "CAM.GenerateCounterbore" // counterbore / spot-face
	GenerateTappingCommandID     = "CAM.GenerateTapping"     // rigid tapping (internal threads)
	GenerateCountersinkCommandID = "CAM.GenerateCountersink" // countersink (conical recess)
	GenerateMillFaceCommandID    = "CAM.GenerateMillFace"    // face milling
	GenerateEngraveCommandID     = "CAM.GenerateEngrave"     // engraving
	GenerateChamferCommandID     = "CAM.GenerateChamfer"     // edge chamfer / deburr
	GenerateVCarveCommandID      = "CAM.GenerateVCarve"      // V-carve relief
	GenerateSurfaceCommandID     = "CAM.GenerateSurface"     // 3D surface finishing (parallel drop-cutter)
	GenerateCrosshatchCommandID  = "CAM.GenerateCrosshatch"  // 3D surface finishing (crosshatch — two perpendicular pass sets)
	GenerateWaterlineCommandID   = "CAM.GenerateWaterline"   // 3D waterline (constant-Z) finishing
	GenerateAllCommandID         = "CAM.GenerateAll"         // one program over several ops + tools
	PreviewProfileCommandID      = "CAM.PreviewProfile"      // transient toolpath preview (not committed)
	ClearPreviewCommandID        = "CAM.ClearPreview"        // remove the transient toolpath preview
	ShowOperationsCommandID      = "CAM.ShowOperations"      // open the operations browser
	EditOperationCommandID       = "CAM.EditOperation"       // open the operation editor
	RegenerateCommandID          = "CAM.RegenerateJob"       // re-run + re-post the edited job
	ToggleOpCommandID            = "CAM.ToggleOperation"     // enable/disable the selected operation
	MoveOpUpCommandID            = "CAM.MoveOperationUp"     // move the selected operation earlier
	MoveOpDownCommandID          = "CAM.MoveOperationDown"   // move the selected operation later
	DeleteOpCommandID            = "CAM.DeleteOperation"     // remove the selected operation
	DuplicateOpCommandID         = "CAM.DuplicateOperation"  // copy the selected operation
	AddTabsCommandID             = "CAM.AddTabs"             // add holding tabs to the selected operation
	AddDogboneCommandID          = "CAM.AddDogbone"          // add dogbone relief to the selected operation
	AddRampCommandID             = "CAM.AddRamp"             // add ramp entry to the selected operation
	AddLeadInOutCommandID        = "CAM.AddLeadInOut"        // add lead-in/out to the selected operation
	AddHelicalRampCommandID      = "CAM.AddHelicalRamp"      // add helical ramp entry to the selected operation
	ClearDressupsCommandID       = "CAM.ClearDressups"       // remove the selected operation's dressups
	ShowToolsCommandID           = "CAM.ShowTools"           // open the tool-library browser
	AddEndmillCommandID          = "CAM.AddEndmill"          // add an end mill to the library
	AddDrillCommandID            = "CAM.AddDrill"            // add a drill to the library
	AddBallnoseCommandID         = "CAM.AddBallnose"         // add a ball-nose to the library
	RemoveToolCommandID          = "CAM.RemoveTool"          // remove the last library tool
	ExportToolsCommandID         = "CAM.ExportTools"         // export the tool library to a file
	ImportToolsCommandID         = "CAM.ImportTools"         // import a tool library from a file
	SaveJobCommandID             = "CAM.SaveJob"             // persist the job into the document
	LoadJobCommandID             = "CAM.LoadJob"             // load the job from the document
	SaveGCodeCommandID           = "CAM.SaveGCode"           // export the posted program to a file
)

// camCommands describes each registered command for registration + the panel.
var camCommands = []struct{ id, name, tip string }{
	{GenerateJobCommandID, "Generate Drilling Job", "Detect the part's holes, generate a drilling toolpath, and post it to G-code."},
	{GenerateProfileCommandID, "Generate Profile Job", "Contour the part's outline with tool compensation, and post it to G-code."},
	{GeneratePocketCommandID, "Generate Pocket Job", "Clear the part's outline region with concentric passes, and post it to G-code."},
	{GenerateAdaptiveCommandID, "Generate Adaptive Job", "Clear the part's outline region with a high-speed low-engagement spiral, and post it to G-code."},
	{GenerateRestCommandID, "Generate Rest Job", "Clear only the wall stock a previous larger tool left behind, and post it to G-code."},
	{GenerateTrochoidalCommandID, "Generate Trochoidal Job", "Mill the part's outline with overlapping trochoidal loops (low engagement), and post it to G-code."},
	{GenerateSlotCommandID, "Generate Slot Job", "Cut a channel of a set width centred on the part's outline, and post it to G-code."},
	{GenerateProbeCommandID, "Generate Probe Job", "Probe the stock top and two edges to find the work origin (G38.2), and post it to G-code."},
	{GenerateBoreProbeCommandID, "Generate Bore Probe Job", "Probe each hole's wall in four directions to find its centre (G38.2), and post it to G-code."},
	{GenerateBossProbeCommandID, "Generate Boss Probe Job", "Probe the part outline's walls inward from four sides to find the footprint centre, and post it to G-code."},
	{GenerateToolProbeCommandID, "Generate Tool Probe Job", "Measure the tool against the tool-setter and set its length offset (G10 L1), and post it to G-code."},
	{GenerateHelixCommandID, "Generate Helix Job", "Bore the part's holes with a helix (for holes wider than the tool)."},
	{GenerateThreadMillCommandID, "Generate Thread Job", "Thread-mill the part's holes by helical interpolation."},
	{GenerateCounterboreCommandID, "Generate Counterbore Job", "Spot-face a flat-bottom recess at each hole top for a screw head."},
	{GenerateTappingCommandID, "Generate Tapping Job", "Cut internal threads in the part's holes with a synchronised tap cycle (G84/G74)."},
	{GenerateCountersinkCommandID, "Generate Countersink Job", "Cut a conical recess at each hole top for a flat-head screw."},
	{GenerateMillFaceCommandID, "Generate Face Job", "Face the top of the stock over the part's outline."},
	{GenerateEngraveCommandID, "Generate Engrave Job", "Engrave the part's outline on the tool centre."},
	{GenerateChamferCommandID, "Generate Chamfer Job", "Break (bevel) the part's top edge with a V-tool chamfer pass."},
	{GenerateVCarveCommandID, "Generate V-Carve Job", "Carve the part's outline region with a V-bit (depth deepens toward the spine)."},
	{GenerateSurfaceCommandID, "Generate Surface Job", "Finish the part's 3D surface with a ball-nose end mill (parallel drop-cutter passes)."},
	{GenerateCrosshatchCommandID, "Generate Crosshatch Job", "Finish the part's 3D surface with two perpendicular pass sets for a finer scallop."},
	{GenerateWaterlineCommandID, "Generate Waterline Job", "Finish the part's 3D surface with constant-Z (waterline) passes — good for steep walls."},
	{GenerateAllCommandID, "Generate All Operations", "Generate one program that drills, contours, and surface-finishes the part — with tool changes between operations."},
	{PreviewProfileCommandID, "Preview Profile", "Show the profile toolpath as a live overlay without committing or posting it."},
	{ClearPreviewCommandID, "Clear Preview", "Remove the live toolpath preview overlay."},
	{ShowOperationsCommandID, "Show Operations", "Open the CAM operations browser for the last generated job."},
	{EditOperationCommandID, "Edit Operation", "Open the operation editor to change a generated operation's parameters."},
	{RegenerateCommandID, "Regenerate Job", "Re-run and re-post the job after editing its operations."},
	{ToggleOpCommandID, "Enable/Disable Operation", "Toggle whether the selected operation runs."},
	{MoveOpUpCommandID, "Move Operation Up", "Run the selected operation earlier in the program."},
	{MoveOpDownCommandID, "Move Operation Down", "Run the selected operation later in the program."},
	{DeleteOpCommandID, "Delete Operation", "Remove the selected operation from the job."},
	{DuplicateOpCommandID, "Duplicate Operation", "Insert a copy of the selected operation."},
	{AddTabsCommandID, "Add Holding Tabs", "Add holding tabs to the selected operation."},
	{AddDogboneCommandID, "Add Dogbone", "Add dogbone corner relief to the selected operation."},
	{AddRampCommandID, "Add Ramp Entry", "Replace straight plunges with a ramped descent on the selected operation."},
	{AddLeadInOutCommandID, "Add Lead In/Out", "Ease the tool into and out of each cut with tangential arcs on the selected operation."},
	{AddHelicalRampCommandID, "Add Helical Ramp", "Replace straight plunges with a helical descent on the selected operation."},
	{ClearDressupsCommandID, "Clear Dressups", "Remove the selected operation's dressups."},
	{ShowToolsCommandID, "Show Tool Library", "Open the CAM tool-library browser."},
	{AddEndmillCommandID, "Add End Mill", "Add an end mill to the tool library."},
	{AddDrillCommandID, "Add Drill", "Add a drill to the tool library."},
	{AddBallnoseCommandID, "Add Ball-nose", "Add a ball-nose cutter to the tool library."},
	{RemoveToolCommandID, "Remove Tool", "Remove the last tool added to the library."},
	{ExportToolsCommandID, "Export Tool Library", "Save the tool library to a JSON file."},
	{ImportToolsCommandID, "Import Tool Library", "Load a tool library from a JSON file."},
	{SaveJobCommandID, "Save CAM Job", "Persist the CAM job into the active document."},
	{LoadJobCommandID, "Load CAM Job", "Load the CAM job stored in the active document."},
	{SaveGCodeCommandID, "Save G-code", "Export the last posted program to a .nc file."},
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
		if json.Unmarshal(ev, &p) == nil {
			switch p.WindowId {
			case CAMPanelID:
				e.applyPanelEdit(p.ControlId, p.Value)
			case OpEditorID:
				e.handleOpEditorEdit(p.ControlId, p.Value)
			}
		}
	case wire.EventFileDialogChosen:
		e.handleFileChosen(ev)
	}
}

// dispatchCommand maps a fired command id to the job that produces its toolpath, ignoring
// commands the add-in does not own.
func (e *Engine) dispatchCommand(commandID string) {
	body := e.body()
	switch commandID {
	case GenerateJobCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunDrillingJobOnHost(body) })
	case GenerateProfileCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunProfileJobOnHost(body) })
	case GeneratePocketCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunPocketJobOnHost(body) })
	case GenerateAdaptiveCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunAdaptiveJobOnHost(body) })
	case GenerateRestCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunRestJobOnHost(body) })
	case GenerateTrochoidalCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunTrochoidalJobOnHost(body) })
	case GenerateSlotCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunSlotJobOnHost(body) })
	case GenerateProbeCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunProbeJobOnHost(body) })
	case GenerateBoreProbeCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunBoreProbeJobOnHost(body) })
	case GenerateBossProbeCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunBossProbeJobOnHost(body) })
	case GenerateToolProbeCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunToolLengthProbeJobOnHost(body) })
	case GenerateHelixCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunHelixJobOnHost(body) })
	case GenerateThreadMillCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunThreadMillJobOnHost(body) })
	case GenerateCounterboreCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunCounterboreJobOnHost(body) })
	case GenerateTappingCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunTappingJobOnHost(body) })
	case GenerateCountersinkCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunCountersinkJobOnHost(body) })
	case GenerateMillFaceCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunMillFaceJobOnHost(body) })
	case GenerateEngraveCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunEngraveJobOnHost(body) })
	case GenerateChamferCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunChamferJobOnHost(body) })
	case GenerateVCarveCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunVCarveJobOnHost(body) })
	case GenerateSurfaceCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunSurface3DJobOnHost(body) })
	case GenerateCrosshatchCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunCrosshatchSurfaceJobOnHost(body) })
	case GenerateWaterlineCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunWaterlineJobOnHost(body) })
	case GenerateAllCommandID:
		e.launchRun(func() (*JobResult, error) { return e.RunAllJobsOnHost(body) })
	case EditOperationCommandID:
		e.launchRun(e.showOperationEditorAction)
	case RegenerateCommandID:
		e.launchRun(e.regenerateAction)
	case ToggleOpCommandID:
		e.launchRun(e.toggleOpAction)
	case MoveOpUpCommandID:
		e.launchRun(e.moveOpUpAction)
	case MoveOpDownCommandID:
		e.launchRun(e.moveOpDownAction)
	case DeleteOpCommandID:
		e.launchRun(e.deleteOpAction)
	case DuplicateOpCommandID:
		e.launchRun(e.duplicateOpAction)
	case AddTabsCommandID:
		e.launchRun(e.addTabsAction)
	case AddDogboneCommandID:
		e.launchRun(e.addDogboneAction)
	case AddRampCommandID:
		e.launchRun(e.addRampAction)
	case AddLeadInOutCommandID:
		e.launchRun(e.addLeadInOutAction)
	case AddHelicalRampCommandID:
		e.launchRun(e.addHelicalRampAction)
	case ClearDressupsCommandID:
		e.launchRun(e.clearDressupsAction)
	case ShowToolsCommandID:
		e.launchRun(e.showToolLibraryAction)
	case AddEndmillCommandID:
		e.launchRun(func() (*JobResult, error) { return e.addToolAction("endmill") })
	case AddDrillCommandID:
		e.launchRun(func() (*JobResult, error) { return e.addToolAction("drill") })
	case AddBallnoseCommandID:
		e.launchRun(func() (*JobResult, error) { return e.addToolAction("ballend") })
	case RemoveToolCommandID:
		e.launchRun(e.removeToolAction)
	case ExportToolsCommandID:
		e.launchRun(e.exportToolsAction)
	case ImportToolsCommandID:
		e.launchRun(e.importToolsAction)
	case PreviewProfileCommandID:
		e.launchRun(func() (*JobResult, error) { return e.PreviewProfileOnHost(0) })
	case ClearPreviewCommandID:
		e.launchRun(e.clearPreviewAction)
	case ShowOperationsCommandID:
		e.launchRun(e.showOperationsAction)
	case SaveJobCommandID:
		e.launchRun(e.saveJobAction)
	case LoadJobCommandID:
		e.launchRun(e.loadJobAction)
	case SaveGCodeCommandID:
		e.launchRun(e.saveGCodeAction)
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

// body returns the index of the body the generate commands machine (set from the panel).
func (e *Engine) body() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.targetBody
}

// reportStatus surfaces a job's outcome on the host status bar (best-effort).
func (e *Engine) reportStatus(msg string) { _, _ = e.api.Status().SetText(msg) }

// JobResult summarizes one generated job.
type JobResult struct {
	GCode            string
	HoleCount        int // drilling only
	GCodeLines       int
	OverlayID        string
	EstimatedMinutes float64 // estimated cycle time
	Summary          string  // human status line
}

// withEstimate appends a "~N.N min" cycle-time note to a summary.
func withEstimate(summary string, minutes float64) string {
	return fmt.Sprintf("%s ~%.1f min.", summary, minutes)
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
	e.mu.Lock()
	estimate := e.lastEstimate
	e.mu.Unlock()
	return &JobResult{
		GCode:            gcodeText,
		HoleCount:        len(holes),
		GCodeLines:       lines,
		OverlayID:        overlayID,
		EstimatedMinutes: estimate,
		Summary:          withEstimate(fmt.Sprintf("CAM: drilled %d hole(s), %d G-code lines (%s).", len(holes), lines, e.postName), estimate),
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
	e.lastEstimate = EstimateMinutes(results)
	e.mu.Unlock()
	gcodeText, err := post.Export(job.PostProcessor, PostObjects(results), "--no-show-editor")
	if err == nil {
		e.rememberGCode(gcodeText)
	}
	return gcodeText, err
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
