// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "fmt"

// toggleOpAction enables or disables the operation selected in the editor (inactive operations
// are skipped when the job is generated).
func (e *Engine) toggleOpAction() (*JobResult, error) {
	return e.mutateSelectedOp(func(job *Job, idx int) string {
		op := job.Operations[idx]
		op.SetActive(!op.Active())
		return fmt.Sprintf("CAM: %q %s.", op.Label(), enabledWord(op.Active()))
	})
}

// moveOpUpAction moves the selected operation one earlier in the program order.
func (e *Engine) moveOpUpAction() (*JobResult, error) {
	return e.mutateSelectedOp(func(job *Job, idx int) string {
		if idx == 0 {
			return "CAM: operation is already first."
		}
		job.Operations[idx-1], job.Operations[idx] = job.Operations[idx], job.Operations[idx-1]
		e.editingOp = idx - 1
		return fmt.Sprintf("CAM: moved %q earlier.", job.Operations[idx-1].Label())
	})
}

// moveOpDownAction moves the selected operation one later in the program order.
func (e *Engine) moveOpDownAction() (*JobResult, error) {
	return e.mutateSelectedOp(func(job *Job, idx int) string {
		if idx >= len(job.Operations)-1 {
			return "CAM: operation is already last."
		}
		job.Operations[idx+1], job.Operations[idx] = job.Operations[idx], job.Operations[idx+1]
		e.editingOp = idx + 1
		return fmt.Sprintf("CAM: moved %q later.", job.Operations[idx+1].Label())
	})
}

// duplicateOpAction inserts a copy of the selected operation right after it and selects it.
func (e *Engine) duplicateOpAction() (*JobResult, error) {
	return e.mutateSelectedOp(func(job *Job, idx int) string {
		clone := job.Operations[idx].Clone()
		next := make([]Operation, 0, len(job.Operations)+1)
		next = append(next, job.Operations[:idx+1]...)
		next = append(next, clone)
		next = append(next, job.Operations[idx+1:]...)
		job.Operations = next
		e.editingOp = idx + 1
		return fmt.Sprintf("CAM: duplicated %q (%d operation(s)).", clone.Label(), len(job.Operations))
	})
}

// defaultCustomGCode is the sample G-code a freshly added custom operation carries, to be edited.
const defaultCustomGCode = "(custom G-code — edit me)\nM0"

// addCustomOpAction inserts a custom (raw G-code) operation right after the selected one, taking
// the selected op's tool controller so it slots into the program without a spurious tool change.
func (e *Engine) addCustomOpAction() (*JobResult, error) {
	return e.mutateSelectedOp(func(job *Job, idx int) string {
		base := OpBase{OpLabel: "Custom", IsActive: true, ToolController: job.Operations[idx].ToolControllerIndex()}
		custom := &CustomOp{OpBase: base, GCode: defaultCustomGCode}
		next := make([]Operation, 0, len(job.Operations)+1)
		next = append(next, job.Operations[:idx+1]...)
		next = append(next, custom)
		next = append(next, job.Operations[idx+1:]...)
		job.Operations = next
		e.editingOp = idx + 1
		return fmt.Sprintf("CAM: added a custom operation (%d operation(s)).", len(job.Operations))
	})
}

// deleteOpAction removes the selected operation from the job.
func (e *Engine) deleteOpAction() (*JobResult, error) {
	return e.mutateSelectedOp(func(job *Job, idx int) string {
		label := job.Operations[idx].Label()
		job.Operations = append(job.Operations[:idx], job.Operations[idx+1:]...)
		if e.editingOp >= len(job.Operations) {
			e.editingOp = len(job.Operations) - 1
		}
		if e.editingOp < 0 {
			e.editingOp = 0
		}
		return fmt.Sprintf("CAM: deleted %q (%d operation(s) left).", label, len(job.Operations))
	})
}

// mutateSelectedOp applies a mutation to the editor's selected operation under the lock, then
// refreshes the operation editor and operations browser. It errors when there is no job.
func (e *Engine) mutateSelectedOp(mutate func(job *Job, idx int) string) (*JobResult, error) {
	e.mu.Lock()
	job, idx := e.lastJob, e.editingOp
	if job == nil || len(job.Operations) == 0 {
		e.mu.Unlock()
		return nil, fmt.Errorf("no operation to edit — generate a job first")
	}
	if idx < 0 || idx >= len(job.Operations) {
		idx = 0
		e.editingOp = 0
	}
	summary := mutate(job, idx)
	editIdx := e.editingOp
	e.mu.Unlock()

	if _, err := e.ShowOperationEditor(job, editIdx); err != nil {
		return nil, err
	}
	if _, err := e.ShowOperationsBrowser(job); err != nil {
		return nil, err
	}
	return &JobResult{Summary: summary}, nil
}

// enabledWord describes an operation's active state for the status bar.
func enabledWord(active bool) string {
	if active {
		return "enabled"
	}
	return "disabled"
}
