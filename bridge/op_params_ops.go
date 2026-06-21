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
		numberParam("roughingPasses", "Roughing passes", float64(op.RoughingPasses)),
		numberParam("roughStep", "Roughing step (mm)", op.RoughStep),
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
	case "roughingPasses":
		op.RoughingPasses = int(panelNum(value, float64(op.RoughingPasses)))
	case "roughStep":
		op.RoughStep = panelNum(value, op.RoughStep)
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
		numberParam("finishAllowance", "Finish allowance (mm)", op.FinishAllowance),
		choiceParam("pattern", "Pattern", pocketPatternOf(op.Pattern), gen.PocketOffset, gen.PocketZigzag),
		boolParam("oneWay", "One-direction (zigzag)", op.OneWay),
		boolParam("climb", "Climb", op.Climb),
	}, depthParams(op.OpBase)...)
}

// pocketPatternOf defaults an unset pocket pattern to the offset (concentric) pattern.
func pocketPatternOf(pattern string) string {
	if pattern == "" {
		return gen.PocketOffset
	}
	return pattern
}

// SetParameter applies one pocket parameter edit.
func (op *PocketOp) SetParameter(id, value string) bool {
	switch id {
	case "stepOver":
		op.StepOver = panelNum(value, op.StepOver)
	case "stepDown":
		op.StepDown = panelNum(value, op.StepDown)
	case "finishAllowance":
		op.FinishAllowance = panelNum(value, op.FinishAllowance)
	case "pattern":
		op.Pattern = value
	case "oneWay":
		op.OneWay = parseBool(value)
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
		numberParam("finishAllowance", "Finish allowance (mm)", op.FinishAllowance),
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
	case "finishAllowance":
		op.FinishAllowance = panelNum(value, op.FinishAllowance)
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

// --- Probe ---

// Parameters lists the editable probe parameters.
func (op *ProbeOp) Parameters() []OpParam {
	return append([]OpParam{
		numberParam("probeFeed", "Probe feed (mm/min)", op.ProbeFeed),
		numberParam("workOffset", "Work offset (1=G54…6=G59)", float64(op.WorkOffset)),
	}, depthParams(op.OpBase)...)
}

// SetParameter applies one probe parameter edit.
func (op *ProbeOp) SetParameter(id, value string) bool {
	switch id {
	case "probeFeed":
		op.ProbeFeed = panelNum(value, op.ProbeFeed)
	case "workOffset":
		op.WorkOffset = int(panelNum(value, float64(op.WorkOffset)))
	default:
		return setDepthParam(&op.OpBase, id, value)
	}
	return true
}

// --- Tool-length probe ---

// Parameters lists the editable tool-length probe parameters.
func (op *ToolLengthProbeOp) Parameters() []OpParam {
	return append([]OpParam{
		numberParam("setterX", "Setter X (mm)", op.SetterX),
		numberParam("setterY", "Setter Y (mm)", op.SetterY),
		numberParam("setterTop", "Setter top Z (mm)", op.SetterTop),
		numberParam("toolNumber", "Tool number", float64(op.ToolNumber)),
		numberParam("probeFeed", "Probe feed (mm/min)", op.ProbeFeed),
	}, depthParams(op.OpBase)...)
}

// SetParameter applies one tool-length probe parameter edit.
func (op *ToolLengthProbeOp) SetParameter(id, value string) bool {
	switch id {
	case "setterX":
		op.SetterX = panelNum(value, op.SetterX)
	case "setterY":
		op.SetterY = panelNum(value, op.SetterY)
	case "setterTop":
		op.SetterTop = panelNum(value, op.SetterTop)
	case "toolNumber":
		op.ToolNumber = int(panelNum(value, float64(op.ToolNumber)))
	case "probeFeed":
		op.ProbeFeed = panelNum(value, op.ProbeFeed)
	default:
		return setDepthParam(&op.OpBase, id, value)
	}
	return true
}

// --- Drilling ---

// Parameters lists the editable drilling parameters.
func (op *DrillingOp) Parameters() []OpParam {
	return append([]OpParam{
		numberParam("drillDepth", "Drill depth (mm, 0=thru)", op.Depth),
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
	case "drillDepth":
		op.Depth = panelNum(value, op.Depth)
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
		boolParam("spiral", "Spiral pattern", op.Spiral),
	}, depthParams(op.OpBase)...)
}

// SetParameter applies one facing parameter edit.
func (op *MillFaceOp) SetParameter(id, value string) bool {
	switch id {
	case "stepOver":
		op.StepOver = panelNum(value, op.StepOver)
	case "stepDown":
		op.StepDown = panelNum(value, op.StepDown)
	case "spiral":
		op.Spiral = parseBool(value)
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

// --- V-carve ---

// Parameters lists the editable V-carve parameters.
func (op *VCarveOp) Parameters() []OpParam {
	return append([]OpParam{
		numberParam("toolAngle", "Tool angle (°)", op.ToolAngle),
		numberParam("stepOver", "Step-over (×⌀)", op.StepOver),
	}, depthParams(op.OpBase)...)
}

// SetParameter applies one V-carve parameter edit.
func (op *VCarveOp) SetParameter(id, value string) bool {
	switch id {
	case "toolAngle":
		op.ToolAngle = panelNum(value, op.ToolAngle)
	case "stepOver":
		op.StepOver = panelNum(value, op.StepOver)
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

// --- Counterbore ---

// Parameters lists the editable counterbore parameters.
func (op *CounterboreOp) Parameters() []OpParam {
	return append([]OpParam{
		numberParam("diameter", "Recess ⌀ (mm)", op.Diameter),
		numberParam("depth", "Recess depth (mm)", op.Depth),
		numberParam("pitch", "Pitch (mm/turn)", op.Pitch),
	}, depthParams(op.OpBase)...)
}

// SetParameter applies one counterbore parameter edit.
func (op *CounterboreOp) SetParameter(id, value string) bool {
	switch id {
	case "diameter":
		op.Diameter = panelNum(value, op.Diameter)
	case "depth":
		op.Depth = panelNum(value, op.Depth)
	case "pitch":
		op.Pitch = panelNum(value, op.Pitch)
	default:
		return setDepthParam(&op.OpBase, id, value)
	}
	return true
}

// --- Tapping ---

// Parameters lists the editable tapping parameters.
func (op *TappingOp) Parameters() []OpParam {
	return append([]OpParam{
		numberParam("pitch", "Thread pitch (mm)", op.Pitch),
		boolParam("lefthand", "Left-hand thread", op.LeftHand),
		numberParam("dwell", "Bottom dwell (s)", op.DwellTime),
	}, depthParams(op.OpBase)...)
}

// SetParameter applies one tapping parameter edit.
func (op *TappingOp) SetParameter(id, value string) bool {
	switch id {
	case "pitch":
		op.Pitch = panelNum(value, op.Pitch)
	case "lefthand":
		op.LeftHand = parseBool(value)
	case "dwell":
		op.DwellTime = panelNum(value, op.DwellTime)
	default:
		return setDepthParam(&op.OpBase, id, value)
	}
	return true
}

// --- Countersink ---

// Parameters lists the editable countersink parameters.
func (op *CountersinkOp) Parameters() []OpParam {
	return append([]OpParam{
		numberParam("diameter", "Rim ⌀ (mm)", op.Diameter),
		numberParam("toolAngle", "Tool angle (°)", op.ToolAngle),
	}, depthParams(op.OpBase)...)
}

// SetParameter applies one countersink parameter edit.
func (op *CountersinkOp) SetParameter(id, value string) bool {
	switch id {
	case "diameter":
		op.Diameter = panelNum(value, op.Diameter)
	case "toolAngle":
		op.ToolAngle = panelNum(value, op.ToolAngle)
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
