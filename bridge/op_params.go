// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "strconv"

// OpParam is one editable parameter of an operation, surfaced in the operation editor. Kind
// selects the control: "number"/"text" render a text box, "choice" a dropdown over Choices.
type OpParam struct {
	ID      string
	Label   string
	Value   string
	Kind    string
	Choices []string
}

// Editable is an operation whose parameters can be listed and edited in the operation editor.
// Operations that do not implement it are shown read-only.
type Editable interface {
	Operation
	Parameters() []OpParam
	SetParameter(id, value string) bool
}

// numberParam builds a numeric parameter row.
func numberParam(id, label string, value float64) OpParam {
	return OpParam{ID: id, Label: label, Value: formatParamNumber(value), Kind: "number"}
}

// choiceParam builds a dropdown parameter row.
func choiceParam(id, label, value string, choices ...string) OpParam {
	return OpParam{ID: id, Label: label, Value: value, Kind: "choice", Choices: choices}
}

// boolParam builds a yes/no dropdown parameter row.
func boolParam(id, label string, value bool) OpParam {
	return choiceParam(id, label, boolWord(value), "yes", "no")
}

// formatParamNumber renders a number without trailing zeros for the editor field.
func formatParamNumber(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

// boolWord maps a bool to the yes/no dropdown value.
func boolWord(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// parseBool reads a yes/no dropdown value.
func parseBool(value string) bool { return value == "yes" }

// depthParams are the start/final-depth, clearance, and coolant rows common to every
// milling/drilling operation.
func depthParams(b OpBase) []OpParam {
	return []OpParam{
		numberParam("startDepth", "Start depth (mm)", b.StartDepth),
		numberParam("finalDepth", "Final depth (mm)", b.FinalDepth),
		numberParam("clearance", "Clearance (mm)", b.ClearanceHeight),
		choiceParam("coolant", "Coolant", b.CoolantMode(), CoolantNone, CoolantFlood, CoolantMist),
	}
}

// setDepthParam applies one common depth/height/coolant edit, reporting whether it matched.
func setDepthParam(b *OpBase, id, value string) bool {
	switch id {
	case "startDepth":
		b.StartDepth = panelNum(value, b.StartDepth)
	case "finalDepth":
		b.FinalDepth = panelNum(value, b.FinalDepth)
	case "clearance":
		b.ClearanceHeight = panelNum(value, b.ClearanceHeight)
	case "coolant":
		b.Coolant = value
	default:
		return false
	}
	return true
}
