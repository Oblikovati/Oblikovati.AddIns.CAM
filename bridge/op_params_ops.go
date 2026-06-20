// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "oblikovati.org/cam/bridge/gen"

// --- Profile ---

// Parameters lists the editable profile parameters.
func (op *ProfileOp) Parameters() []OpParam {
	return append([]OpParam{
		choiceParam("side", "Side", op.Side, gen.SideOutside, gen.SideInside, gen.SideOn),
		numberParam("offsetExtra", "Extra stock (mm)", op.OffsetExtra),
		numberParam("stepDown", "Step-down (mm)", op.StepDown),
		boolParam("climb", "Climb", op.Climb),
	}, depthParams(op.OpBase)...)
}

// SetParameter applies one profile parameter edit.
func (op *ProfileOp) SetParameter(id, value string) bool {
	switch id {
	case "side":
		op.Side = value
	case "offsetExtra":
		op.OffsetExtra = panelNum(value, op.OffsetExtra)
	case "stepDown":
		op.StepDown = panelNum(value, op.StepDown)
	case "climb":
		op.Climb = parseBool(value)
	default:
		return setDepthParam(&op.OpBase, id, value)
	}
	return true
}

// --- Pocket ---

// Parameters lists the editable pocket parameters.
func (op *PocketOp) Parameters() []OpParam {
	return append([]OpParam{
		numberParam("stepOver", "Step-over (×⌀)", op.StepOver),
		numberParam("stepDown", "Step-down (mm)", op.StepDown),
		boolParam("climb", "Climb", op.Climb),
	}, depthParams(op.OpBase)...)
}

// SetParameter applies one pocket parameter edit.
func (op *PocketOp) SetParameter(id, value string) bool {
	switch id {
	case "stepOver":
		op.StepOver = panelNum(value, op.StepOver)
	case "stepDown":
		op.StepDown = panelNum(value, op.StepDown)
	case "climb":
		op.Climb = parseBool(value)
	default:
		return setDepthParam(&op.OpBase, id, value)
	}
	return true
}

// --- Adaptive ---

// Parameters lists the editable adaptive clearing parameters.
func (op *AdaptiveOp) Parameters() []OpParam {
	return append([]OpParam{
		numberParam("stepOver", "Engagement (×⌀)", op.StepOver),
		numberParam("stepDown", "Step-down (mm)", op.StepDown),
		boolParam("climb", "Climb", op.Climb),
	}, depthParams(op.OpBase)...)
}

// SetParameter applies one adaptive clearing parameter edit.
func (op *AdaptiveOp) SetParameter(id, value string) bool {
	switch id {
	case "stepOver":
		op.StepOver = panelNum(value, op.StepOver)
	case "stepDown":
		op.StepDown = panelNum(value, op.StepDown)
	case "climb":
		op.Climb = parseBool(value)
	default:
		return setDepthParam(&op.OpBase, id, value)
	}
	return true
}

// --- Rest ---

// Parameters lists the editable rest-machining parameters.
func (op *RestOp) Parameters() []OpParam {
	return append([]OpParam{
		numberParam("prevTool", "Previous tool ⌀ (mm)", op.PrevToolDiameter),
		numberParam("stepOver", "Step-over (×⌀)", op.StepOver),
		numberParam("stepDown", "Step-down (mm)", op.StepDown),
		boolParam("climb", "Climb", op.Climb),
	}, depthParams(op.OpBase)...)
}

// SetParameter applies one rest-machining parameter edit.
func (op *RestOp) SetParameter(id, value string) bool {
	switch id {
	case "prevTool":
		op.PrevToolDiameter = panelNum(value, op.PrevToolDiameter)
	case "stepOver":
		op.StepOver = panelNum(value, op.StepOver)
	case "stepDown":
		op.StepDown = panelNum(value, op.StepDown)
	case "climb":
		op.Climb = parseBool(value)
	default:
		return setDepthParam(&op.OpBase, id, value)
	}
	return true
}

// --- Slot ---

// Parameters lists the editable slot parameters.
func (op *SlotOp) Parameters() []OpParam {
	return append([]OpParam{
		numberParam("width", "Width (mm)", op.Width),
		numberParam("stepOver", "Step-over (×⌀)", op.StepOver),
		numberParam("stepDown", "Step-down (mm)", op.StepDown),
		boolParam("climb", "Climb", op.Climb),
	}, depthParams(op.OpBase)...)
}

// SetParameter applies one slot parameter edit.
func (op *SlotOp) SetParameter(id, value string) bool {
	switch id {
	case "width":
		op.Width = panelNum(value, op.Width)
	case "stepOver":
		op.StepOver = panelNum(value, op.StepOver)
	case "stepDown":
		op.StepDown = panelNum(value, op.StepDown)
	case "climb":
		op.Climb = parseBool(value)
	default:
		return setDepthParam(&op.OpBase, id, value)
	}
	return true
}

// --- Trochoidal ---

// Parameters lists the editable trochoidal parameters.
func (op *TrochoidalOp) Parameters() []OpParam {
	return append([]OpParam{
		numberParam("loopRadius", "Loop radius (mm)", op.LoopRadius),
		numberParam("advance", "Advance (mm)", op.Advance),
		choiceParam("side", "Side", op.Side, gen.SideOutside, gen.SideInside, gen.SideOn),
		numberParam("stepDown", "Step-down (mm)", op.StepDown),
	}, depthParams(op.OpBase)...)
}

// SetParameter applies one trochoidal parameter edit.
func (op *TrochoidalOp) SetParameter(id, value string) bool {
	switch id {
	case "loopRadius":
		op.LoopRadius = panelNum(value, op.LoopRadius)
	case "advance":
		op.Advance = panelNum(value, op.Advance)
	case "side":
		op.Side = value
	case "stepDown":
		op.StepDown = panelNum(value, op.StepDown)
	default:
		return setDepthParam(&op.OpBase, id, value)
	}
	return true
}

// --- Drilling ---

// Parameters lists the editable drilling parameters.
func (op *DrillingOp) Parameters() []OpParam {
	return append([]OpParam{
		numberParam("peckDepth", "Peck depth (mm)", op.PeckDepth),
		numberParam("dwellTime", "Dwell (s)", op.DwellTime),
		numberParam("repeat", "Repeat", float64(op.Repeat)),
		boolParam("chipBreak", "Chip-break", op.ChipBreak),
		boolParam("feedRetract", "Feed retract", op.FeedRetract),
	}, depthParams(op.OpBase)...)
}

// SetParameter applies one drilling parameter edit.
func (op *DrillingOp) SetParameter(id, value string) bool {
	switch id {
	case "peckDepth":
		op.PeckDepth = panelNum(value, op.PeckDepth)
	case "dwellTime":
		op.DwellTime = panelNum(value, op.DwellTime)
	case "repeat":
		op.Repeat = int(panelNum(value, float64(op.Repeat)))
	case "chipBreak":
		op.ChipBreak = parseBool(value)
	case "feedRetract":
		op.FeedRetract = parseBool(value)
	default:
		return setDepthParam(&op.OpBase, id, value)
	}
	return true
}

// --- Mill face ---

// Parameters lists the editable facing parameters.
func (op *MillFaceOp) Parameters() []OpParam {
	return append([]OpParam{
		numberParam("stepOver", "Step-over (×⌀)", op.StepOver),
		numberParam("stepDown", "Step-down (mm)", op.StepDown),
	}, depthParams(op.OpBase)...)
}

// SetParameter applies one facing parameter edit.
func (op *MillFaceOp) SetParameter(id, value string) bool {
	switch id {
	case "stepOver":
		op.StepOver = panelNum(value, op.StepOver)
	case "stepDown":
		op.StepDown = panelNum(value, op.StepDown)
	default:
		return setDepthParam(&op.OpBase, id, value)
	}
	return true
}

// --- Engrave ---

// Parameters lists the editable engraving parameters.
func (op *EngraveOp) Parameters() []OpParam {
	return append([]OpParam{boolParam("climb", "Climb", op.Climb)}, depthParams(op.OpBase)...)
}

// SetParameter applies one engraving parameter edit.
func (op *EngraveOp) SetParameter(id, value string) bool {
	if id == "climb" {
		op.Climb = parseBool(value)
		return true
	}
	return setDepthParam(&op.OpBase, id, value)
}

// --- Helix ---

// Parameters lists the editable helix parameters.
func (op *HelixOp) Parameters() []OpParam {
	return append([]OpParam{
		numberParam("holeRadius", "Hole radius (mm)", op.HoleRadius),
		numberParam("pitch", "Pitch (mm/turn)", op.Pitch),
		choiceParam("direction", "Direction", op.Direction, gen.HelixCW, gen.HelixCCW),
	}, depthParams(op.OpBase)...)
}

// SetParameter applies one helix parameter edit.
func (op *HelixOp) SetParameter(id, value string) bool {
	switch id {
	case "holeRadius":
		op.HoleRadius = panelNum(value, op.HoleRadius)
	case "pitch":
		op.Pitch = panelNum(value, op.Pitch)
	case "direction":
		op.Direction = value
	default:
		return setDepthParam(&op.OpBase, id, value)
	}
	return true
}

// --- Chamfer ---

// Parameters lists the editable chamfer parameters.
func (op *ChamferOp) Parameters() []OpParam {
	return append([]OpParam{
		numberParam("width", "Width (mm)", op.Width),
		numberParam("toolAngle", "Tool angle (°)", op.ToolAngle),
		choiceParam("side", "Side", op.Side, gen.SideOutside, gen.SideInside, gen.SideOn),
		boolParam("climb", "Climb", op.Climb),
	}, depthParams(op.OpBase)...)
}

// SetParameter applies one chamfer parameter edit.
func (op *ChamferOp) SetParameter(id, value string) bool {
	switch id {
	case "width":
		op.Width = panelNum(value, op.Width)
	case "toolAngle":
		op.ToolAngle = panelNum(value, op.ToolAngle)
	case "side":
		op.Side = value
	case "climb":
		op.Climb = parseBool(value)
	default:
		return setDepthParam(&op.OpBase, id, value)
	}
	return true
}

// --- Thread mill ---

// Parameters lists the editable thread-milling parameters.
func (op *ThreadMillOp) Parameters() []OpParam {
	return append([]OpParam{
		numberParam("majorDia", "Major ⌀ (mm)", op.MajorDiameter),
		numberParam("pitch", "Pitch (mm/turn)", op.Pitch),
		boolParam("internal", "Internal", op.Internal),
		boolParam("climb", "Climb", op.Climb),
	}, depthParams(op.OpBase)...)
}

// SetParameter applies one thread-milling parameter edit.
func (op *ThreadMillOp) SetParameter(id, value string) bool {
	switch id {
	case "majorDia":
		op.MajorDiameter = panelNum(value, op.MajorDiameter)
	case "pitch":
		op.Pitch = panelNum(value, op.Pitch)
	case "internal":
		op.Internal = parseBool(value)
	case "climb":
		op.Climb = parseBool(value)
	default:
		return setDepthParam(&op.OpBase, id, value)
	}
	return true
}

// --- Surface ---

// Parameters lists the editable 3D-surface parameters.
func (op *SurfaceOp) Parameters() []OpParam {
	return append([]OpParam{
		numberParam("stepOver", "Pass spacing (mm)", op.StepOver),
		numberParam("sampling", "Sampling (mm)", op.Sampling),
		boolParam("zigzag", "Zigzag", op.Zigzag),
	}, depthParams(op.OpBase)...)
}

// SetParameter applies one 3D-surface parameter edit.
func (op *SurfaceOp) SetParameter(id, value string) bool {
	switch id {
	case "stepOver":
		op.StepOver = panelNum(value, op.StepOver)
	case "sampling":
		op.Sampling = panelNum(value, op.Sampling)
	case "zigzag":
		op.Zigzag = parseBool(value)
	default:
		return setDepthParam(&op.OpBase, id, value)
	}
	return true
}

// --- Waterline ---

// Parameters lists the editable waterline parameters.
func (op *WaterlineOp) Parameters() []OpParam {
	return append([]OpParam{
		numberParam("stepOver", "Grid (mm)", op.StepOver),
		numberParam("stepDown", "Level step (mm)", op.StepDown),
	}, depthParams(op.OpBase)...)
}

// SetParameter applies one waterline parameter edit.
func (op *WaterlineOp) SetParameter(id, value string) bool {
	switch id {
	case "stepOver":
		op.StepOver = panelNum(value, op.StepOver)
	case "stepDown":
		op.StepDown = panelNum(value, op.StepDown)
	default:
		return setDepthParam(&op.OpBase, id, value)
	}
	return true
}
