// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "oblikovati.org/api/types"

// camRibbonSpot is where one command sits on the CAM tab: its panel (operation-type group), the
// glyph key (an embedded icons/<key>.svg, "" for a text button), and the button style.
type camRibbonSpot struct {
	panel string
	icon  string
	style types.ButtonStyle
}

// camRibbonSpots places every CAM command on a panel of the CAM tab. The cutting tools are large
// icon buttons with the add-in's own Oblikovati-style glyph; the program, job, window and
// tool-library actions are small icon buttons; the per-operation modify/dress-up actions are compact
// text buttons (they act on the selected operation in the browser). A command absent here would land
// on an unnamed panel, so the map is kept exhaustive — covered by a registration test.
var camRibbonSpots = map[string]camRibbonSpot{
	// 2.5D milling
	GenerateProfileCommandID:    {"2.5D Milling", "profile", types.LargeIconButton},
	GeneratePocketCommandID:     {"2.5D Milling", "pocket", types.LargeIconButton},
	GenerateAdaptiveCommandID:   {"2.5D Milling", "adaptive", types.LargeIconButton},
	GenerateRestCommandID:       {"2.5D Milling", "rest", types.LargeIconButton},
	GenerateTrochoidalCommandID: {"2.5D Milling", "trochoidal", types.LargeIconButton},
	GenerateSlotCommandID:       {"2.5D Milling", "slot", types.LargeIconButton},
	GenerateMillFaceCommandID:   {"2.5D Milling", "face", types.LargeIconButton},
	GenerateEngraveCommandID:    {"2.5D Milling", "engrave", types.LargeIconButton},
	GenerateChamferCommandID:    {"2.5D Milling", "chamfer", types.LargeIconButton},
	GenerateVCarveCommandID:     {"2.5D Milling", "vcarve", types.LargeIconButton},

	// Drilling & holes
	GenerateJobCommandID:         {"Drilling & Holes", "drilling", types.LargeIconButton},
	GenerateHelixCommandID:       {"Drilling & Holes", "helix", types.LargeIconButton},
	GenerateThreadMillCommandID:  {"Drilling & Holes", "threadmill", types.LargeIconButton},
	GenerateCounterboreCommandID: {"Drilling & Holes", "counterbore", types.LargeIconButton},
	GenerateTappingCommandID:     {"Drilling & Holes", "tapping", types.LargeIconButton},
	GenerateCountersinkCommandID: {"Drilling & Holes", "countersink", types.LargeIconButton},

	// 3D surfacing
	GenerateSurfaceCommandID:    {"3D", "surface", types.LargeIconButton},
	GenerateCrosshatchCommandID: {"3D", "crosshatch", types.LargeIconButton},
	GenerateWaterlineCommandID:  {"3D", "waterline", types.LargeIconButton},

	// Probing
	GenerateProbeCommandID:     {"Probing", "probe", types.LargeIconButton},
	GenerateBoreProbeCommandID: {"Probing", "boreprobe", types.LargeIconButton},
	GenerateBossProbeCommandID: {"Probing", "bossprobe", types.LargeIconButton},
	GenerateToolProbeCommandID: {"Probing", "toolprobe", types.LargeIconButton},

	// Program output
	GenerateAllCommandID:    {"Program", "generateall", types.LargeIconButton},
	SaveGCodeCommandID:      {"Program", "savegcode", types.SmallIconButton},
	PreviewProfileCommandID: {"Program", "preview", types.SmallIconButton},
	ClearPreviewCommandID:   {"Program", "clearpreview", types.SmallIconButton},

	// Job persistence
	SaveJobCommandID: {"Job", "savejob", types.SmallIconButton},
	LoadJobCommandID: {"Job", "loadjob", types.SmallIconButton},

	// Windows (open the dockable panels/browsers on demand)
	ShowPanelCommandID:      {"Windows", "campanel", types.SmallIconButton},
	ShowOperationsCommandID: {"Windows", "showops", types.SmallIconButton},
	ShowToolsCommandID:      {"Windows", "toollib", types.SmallIconButton},

	// Modify the selected operation (acts on the operations browser selection)
	RegenerateCommandID:    {"Modify", "", types.TextOnlyButton},
	EditOperationCommandID: {"Modify", "", types.TextOnlyButton},
	ToggleOpCommandID:      {"Modify", "", types.TextOnlyButton},
	MoveOpUpCommandID:      {"Modify", "", types.TextOnlyButton},
	MoveOpDownCommandID:    {"Modify", "", types.TextOnlyButton},
	DeleteOpCommandID:      {"Modify", "", types.TextOnlyButton},
	DuplicateOpCommandID:   {"Modify", "", types.TextOnlyButton},
	AddCustomOpCommandID:   {"Modify", "", types.TextOnlyButton},

	// Dress-ups (added to the selected operation)
	AddTabsCommandID:        {"Dress-up", "", types.TextOnlyButton},
	AddDogboneCommandID:     {"Dress-up", "", types.TextOnlyButton},
	AddRampCommandID:        {"Dress-up", "", types.TextOnlyButton},
	AddLeadInOutCommandID:   {"Dress-up", "", types.TextOnlyButton},
	AddHelicalRampCommandID: {"Dress-up", "", types.TextOnlyButton},
	ClearDressupsCommandID:  {"Dress-up", "", types.TextOnlyButton},

	// Tool library
	AddEndmillCommandID:  {"Tool Library", "", types.TextOnlyButton},
	AddDrillCommandID:    {"Tool Library", "", types.TextOnlyButton},
	AddBallnoseCommandID: {"Tool Library", "", types.TextOnlyButton},
	RemoveToolCommandID:  {"Tool Library", "", types.TextOnlyButton},
	ExportToolsCommandID: {"Tool Library", "", types.TextOnlyButton},
	ImportToolsCommandID: {"Tool Library", "", types.TextOnlyButton},
}
