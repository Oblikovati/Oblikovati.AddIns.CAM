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
