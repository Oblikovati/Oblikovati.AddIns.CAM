// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"oblikovati.org/cam/bridge/dressup"
	"oblikovati.org/cam/bridge/gcode"
)

// Dressup is a post-process applied to an operation's toolpath to add a manufacturing
// feature the raw geometry lacks — holding tabs, dogbone corner relief, … It is the bridge
// adapter over the pure dressup package, letting an operation carry an ordered chain of
// dressups that frame() applies after generating the cut.
type Dressup interface {
	Apply(gcode.Path) gcode.Path // transform the toolpath
	Name() string                // display name (operations browser / persistence)
}

// TagsDressup lifts the tool over evenly spaced holding tabs.
type TagsDressup struct{ Params dressup.TagParams }

// Apply implements Dressup.
func (d TagsDressup) Apply(path gcode.Path) gcode.Path { return dressup.ApplyTags(path, d.Params) }

// Name implements Dressup.
func (d TagsDressup) Name() string { return "Tags" }

// DogboneDressup cuts corner-relief bones so a round end mill reaches internal corners.
type DogboneDressup struct{ Params dressup.DogboneParams }

// Apply implements Dressup.
func (d DogboneDressup) Apply(path gcode.Path) gcode.Path {
	return dressup.ApplyDogbone(path, d.Params)
}

// Name implements Dressup.
func (d DogboneDressup) Name() string { return "Dogbone" }

// RampDressup replaces straight plunges with a ramped descent.
type RampDressup struct{ Params dressup.RampParams }

// Apply implements Dressup.
func (d RampDressup) Apply(path gcode.Path) gcode.Path { return dressup.ApplyRamp(path, d.Params) }

// Name implements Dressup.
func (d RampDressup) Name() string { return "Ramp" }

// NewRampDressup builds a ramp-entry dressup with the given run length (mm) and descent angle
// (radians).
func NewRampDressup(length, angle float64) RampDressup {
	return RampDressup{Params: dressup.RampParams{Length: length, Angle: angle}}
}

// HelicalRampDressup replaces straight plunges with a helical descent on a circle tangent to the
// first cut — the gentle entry for closed pockets without room for a linear ramp.
type HelicalRampDressup struct{ Params dressup.HelicalRampParams }

// Apply implements Dressup.
func (d HelicalRampDressup) Apply(path gcode.Path) gcode.Path {
	return dressup.ApplyHelicalRamp(path, d.Params)
}

// ApplyWithRoom implements roomAwareDressup: the helix radius is shrunk per plunge so the entry
// circle stays inside the wall clearance reported by roomAt, preventing a fixed radius from gouging
// a nearby wall in a tight pocket. Operations that know their boundary supply roomAt.
func (d HelicalRampDressup) ApplyWithRoom(path gcode.Path, roomAt func(x, y float64) float64) gcode.Path {
	return dressup.ApplyHelicalRampBounded(path, d.Params, roomAt)
}

// Name implements Dressup.
func (d HelicalRampDressup) Name() string { return "Helical Ramp" }

// NewHelicalRampDressup builds a helical ramp-entry dressup with the given helix radius (mm) and
// descent pitch (mm per turn).
func NewHelicalRampDressup(radius, pitch float64) HelicalRampDressup {
	return HelicalRampDressup{Params: dressup.HelicalRampParams{Radius: radius, Pitch: pitch}}
}

// LeadInOutDressup eases the tool into and out of each cut with tangential arcs.
type LeadInOutDressup struct{ Params dressup.LeadInOutParams }

// Apply implements Dressup.
func (d LeadInOutDressup) Apply(path gcode.Path) gcode.Path {
	return dressup.ApplyLeadInOut(path, d.Params)
}

// Name implements Dressup.
func (d LeadInOutDressup) Name() string { return "Lead In/Out" }

// NewLeadInOutDressup builds a lead-in/out dressup with the given lead arc radius (mm) on the
// given side (dressup.SideLeft | dressup.SideRight).
func NewLeadInOutDressup(radius float64, side string) LeadInOutDressup {
	return LeadInOutDressup{Params: dressup.LeadInOutParams{Radius: radius, Side: side}}
}

// NewTagsDressup builds a holding-tabs dressup with count tabs of the given width/height (mm).
func NewTagsDressup(count int, width, height float64) TagsDressup {
	return TagsDressup{Params: dressup.TagParams{Count: count, Width: width, Height: height}}
}

// NewDogboneDressup builds a dogbone dressup of the given style and bone length (mm), relieving
// corners turning sharper than minAngle (radians) on the chosen side.
func NewDogboneDressup(style string, length, minAngle float64, side string) DogboneDressup {
	return DogboneDressup{Params: dressup.DogboneParams{Style: style, Length: length, MinAngle: minAngle, Side: side}}
}
