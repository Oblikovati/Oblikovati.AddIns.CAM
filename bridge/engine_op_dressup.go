// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"math"

	"oblikovati.org/cam/bridge/dressup"
)

// addTabsAction adds a holding-tabs dressup to the operation selected in the editor.
func (e *Engine) addTabsAction() (*JobResult, error) {
	return e.addDressup(NewTagsDressup(defaultTabCount, defaultTabWidth, defaultTabHeight), "holding tabs")
}

// addDogboneAction adds a dogbone corner-relief dressup to the selected operation.
func (e *Engine) addDogboneAction() (*JobResult, error) {
	return e.addDressup(NewDogboneDressup(dressup.StyleDogbone, defaultBoneLength, math.Pi/4, dressup.SideBoth), "dogbone")
}

// addRampAction adds a ramp-entry dressup to the selected operation.
func (e *Engine) addRampAction() (*JobResult, error) {
	return e.addDressup(NewRampDressup(defaultRampLength, defaultRampAngle), "ramp entry")
}

// Defaults for dressups added from the editor (mm / radians).
const (
	defaultTabCount   = 4
	defaultTabWidth   = 3.0
	defaultTabHeight  = 1.0
	defaultBoneLength = 2.0
	defaultRampLength = 4.0
	defaultRampAngle  = math.Pi / 12 // 15°
)

// addDressup appends a dressup to the editor's selected operation, refreshing the windows.
func (e *Engine) addDressup(d Dressup, name string) (*JobResult, error) {
	return e.mutateSelectedOp(func(job *Job, idx int) string {
		holder, ok := job.Operations[idx].(dressupHolder)
		if !ok {
			return fmt.Sprintf("CAM: %q takes no dressups.", job.Operations[idx].Label())
		}
		holder.AppendDressup(d)
		return fmt.Sprintf("CAM: added %s to %q (%d dressup(s)).", name, job.Operations[idx].Label(), holder.DressupCount())
	})
}

// clearDressupsAction removes all dressups from the selected operation.
func (e *Engine) clearDressupsAction() (*JobResult, error) {
	return e.mutateSelectedOp(func(job *Job, idx int) string {
		holder, ok := job.Operations[idx].(dressupHolder)
		if !ok {
			return fmt.Sprintf("CAM: %q takes no dressups.", job.Operations[idx].Label())
		}
		holder.ClearDressups()
		return fmt.Sprintf("CAM: cleared dressups on %q.", job.Operations[idx].Label())
	})
}
