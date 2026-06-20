// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/api/client"
	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// OpsBrowserID is the stable dockable-window id of the CAM operations browser.
const OpsBrowserID = "com.oblikovati.cam.operations"

// showOperationsAction opens (or refreshes) the operations browser for the last generated
// job and reports via the status bar. It runs on the job goroutine (it makes a host call).
func (e *Engine) showOperationsAction() (*JobResult, error) {
	e.mu.Lock()
	job := e.lastJob
	e.mu.Unlock()
	if _, err := e.ShowOperationsBrowser(job); err != nil {
		return nil, err
	}
	n := 0
	if job != nil {
		n = len(job.Operations)
	}
	return &JobResult{Summary: fmt.Sprintf("CAM: operations browser open (%d operation(s)).", n)}, nil
}

// ShowOperationsBrowser builds the operations-browser dockable window: one labelled row per
// operation showing its order, kind, label, and active state, plus Save/Load buttons. An
// empty job shows a hint.
func (e *Engine) ShowOperationsBrowser(job *Job) (wire.OKResult, error) {
	controls := []wire.PanelControlSpec{client.PanelLabel("hdr", "— Operations —")}
	if job == nil || len(job.Operations) == 0 {
		controls = append(controls, client.PanelLabel("empty", "No job yet — generate one from the CAM panel."))
	} else {
		for i, op := range job.Operations {
			controls = append(controls, client.PanelLabel(fmt.Sprintf("op%d", i), operationRow(i, op)))
		}
	}
	controls = append(controls,
		client.PanelSeparator(),
		client.PanelButton("save", "Save Job", SaveJobCommandID),
		client.PanelButton("load", "Load Job", LoadJobCommandID),
	)
	return e.api.DockableWindows().Set(wire.DockableWindowSpec{
		ID: OpsBrowserID, Title: "CAM Operations", Dock: types.DockLeft, Visible: true, Controls: controls,
	})
}

// operationRow formats one browser row: "1. Drilling 'Drill' ✓".
func operationRow(index int, op Operation) string {
	state := "✓"
	if !op.Active() {
		state = "—"
	}
	return fmt.Sprintf("%d. %s %q %s", index+1, operationKind(op), op.Label(), state)
}

// operationKind returns a human label for an operation's concrete type.
func operationKind(op Operation) string {
	switch op.(type) {
	case *DrillingOp:
		return "Drilling"
	case *ProfileOp:
		return "Profile"
	case *PocketOp:
		return "Pocket"
	case *AdaptiveOp:
		return "Adaptive"
	case *RestOp:
		return "Rest"
	case *MillFaceOp:
		return "Face"
	case *EngraveOp:
		return "Engrave"
	case *HelixOp:
		return "Helix"
	case *ThreadMillOp:
		return "Thread"
	case *SurfaceOp:
		return "Surface"
	case *WaterlineOp:
		return "Waterline"
	default:
		return "Operation"
	}
}

// saveJobAction persists the last generated job into the active document.
func (e *Engine) saveJobAction() (*JobResult, error) {
	e.mu.Lock()
	job := e.lastJob
	e.mu.Unlock()
	if job == nil {
		return nil, fmt.Errorf("no job to save — generate one first")
	}
	if err := e.SaveJob(job); err != nil {
		return nil, err
	}
	if err := e.SaveToolLibrary(); err != nil {
		return nil, err
	}
	return &JobResult{Summary: fmt.Sprintf("CAM: job saved (%d operation(s)).", len(job.Operations))}, nil
}

// loadJobAction loads the stored job (and tool library) from the active document and makes it
// the current job.
func (e *Engine) loadJobAction() (*JobResult, error) {
	if err := e.LoadToolLibrary(); err != nil {
		return nil, err
	}
	job, err := e.LoadJob()
	if err != nil {
		return nil, err
	}
	if job == nil {
		return &JobResult{Summary: "CAM: no saved job in this document."}, nil
	}
	e.mu.Lock()
	e.lastJob = job
	e.mu.Unlock()
	if _, err := e.ShowOperationsBrowser(job); err != nil {
		return nil, err
	}
	return &JobResult{Summary: fmt.Sprintf("CAM: job loaded (%d operation(s)).", len(job.Operations))}, nil
}
