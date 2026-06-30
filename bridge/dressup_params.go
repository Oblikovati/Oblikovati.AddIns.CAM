// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "oblikovati.org/cam/bridge/dressup"

// DressupEditable is a dressup whose parameters can be listed and edited in the operation editor
// (FreeCAD's per-dressup edit dialogs — HoldingTagsEdit, DogboneEdit, …). Dressups are value types
// carried in an interface slice, so editing returns an updated copy (WithParameter) that the
// operation stores back, rather than mutating in place.
type DressupEditable interface {
	Dressup
	Parameters() []OpParam
	WithParameter(id, value string) Dressup
}

// --- Holding tabs ---

// Parameters lists the editable holding-tab parameters (FreeCAD's HoldingTagsEdit).
func (d TagsDressup) Parameters() []OpParam {
	return []OpParam{
		numberParam("count", "Tabs", float64(d.Params.Count)),
		numberParam("width", "Width (mm)", d.Params.Width),
		numberParam("height", "Height (mm)", d.Params.Height),
	}
}

// WithParameter returns a copy of the holding-tab dressup with one parameter set.
func (d TagsDressup) WithParameter(id, value string) Dressup {
	switch id {
	case "count":
		d.Params.Count = int(panelNum(value, float64(d.Params.Count)))
	case "width":
		d.Params.Width = panelNum(value, d.Params.Width)
	case "height":
		d.Params.Height = panelNum(value, d.Params.Height)
	}
	return d
}

// --- Dogbone ---

func (d DogboneDressup) Parameters() []OpParam {
	return []OpParam{
		choiceParam("style", "Style", d.Params.Style,
			dressup.StyleDogbone, dressup.StyleTBoneLong, dressup.StyleTBoneShrt),
		numberParam("length", "Bone length (mm)", d.Params.Length),
		numberParam("minAngle", "Min angle (rad)", d.Params.MinAngle),
		choiceParam("side", "Side", d.Params.Side, dressup.SideLeft, dressup.SideRight),
	}
}

func (d DogboneDressup) WithParameter(id, value string) Dressup {
	switch id {
	case "style":
		d.Params.Style = value
	case "length":
		d.Params.Length = panelNum(value, d.Params.Length)
	case "minAngle":
		d.Params.MinAngle = panelNum(value, d.Params.MinAngle)
	case "side":
		d.Params.Side = value
	}
	return d
}

// --- Ramp ---

func (d RampDressup) Parameters() []OpParam {
	return []OpParam{
		numberParam("length", "Run length (mm)", d.Params.Length),
		numberParam("angle", "Descent angle (rad)", d.Params.Angle),
	}
}

func (d RampDressup) WithParameter(id, value string) Dressup {
	switch id {
	case "length":
		d.Params.Length = panelNum(value, d.Params.Length)
	case "angle":
		d.Params.Angle = panelNum(value, d.Params.Angle)
	}
	return d
}

// --- Helical ramp ---

func (d HelicalRampDressup) Parameters() []OpParam {
	return []OpParam{
		numberParam("radius", "Helix radius (mm)", d.Params.Radius),
		numberParam("pitch", "Pitch (mm/turn)", d.Params.Pitch),
	}
}

func (d HelicalRampDressup) WithParameter(id, value string) Dressup {
	switch id {
	case "radius":
		d.Params.Radius = panelNum(value, d.Params.Radius)
	case "pitch":
		d.Params.Pitch = panelNum(value, d.Params.Pitch)
	}
	return d
}

// --- Lead in/out ---

func (d LeadInOutDressup) Parameters() []OpParam {
	return []OpParam{
		numberParam("radius", "Lead radius (mm)", d.Params.Radius),
		choiceParam("side", "Side", d.Params.Side, dressup.SideLeft, dressup.SideRight),
	}
}

func (d LeadInOutDressup) WithParameter(id, value string) Dressup {
	switch id {
	case "radius":
		d.Params.Radius = panelNum(value, d.Params.Radius)
	case "side":
		d.Params.Side = value
	}
	return d
}
