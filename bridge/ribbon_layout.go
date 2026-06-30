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

// camRibbonSpots places every CAM command on a panel of the CAM tab. Every command carries the
// add-in's own Oblikovati-style glyph: the frequently used cutting tools are LARGE icon buttons,
// while the less-used program, job, window, modify, dress-up and tool-library actions are SMALL icon
// buttons (an icon beside the label). A command absent here would land on an unnamed panel without a
// glyph, so the map is kept exhaustive and all-icon — covered by TestEveryCommandIsAnIconButton.
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

	// Modify the selected operation (acts on the operations browser selection). These are
	// less-used than the cutting tools, so they are SMALL icon buttons — an icon plus the label.
	RegenerateCommandID:    {"Modify", "regenerate", types.SmallIconButton},
	EditOperationCommandID: {"Modify", "editop", types.SmallIconButton},
	ToggleOpCommandID:      {"Modify", "toggleop", types.SmallIconButton},
	MoveOpUpCommandID:      {"Modify", "moveup", types.SmallIconButton},
	MoveOpDownCommandID:    {"Modify", "movedown", types.SmallIconButton},
	DeleteOpCommandID:      {"Modify", "deleteop", types.SmallIconButton},
	DuplicateOpCommandID:   {"Modify", "duplicateop", types.SmallIconButton},
	AddCustomOpCommandID:   {"Modify", "customop", types.SmallIconButton},

	// Dress-ups (added to the selected operation)
	AddTabsCommandID:        {"Dress-up", "tabs", types.SmallIconButton},
	AddDogboneCommandID:     {"Dress-up", "dogbone", types.SmallIconButton},
	AddRampCommandID:        {"Dress-up", "ramp", types.SmallIconButton},
	AddLeadInOutCommandID:   {"Dress-up", "leadinout", types.SmallIconButton},
	AddHelicalRampCommandID: {"Dress-up", "helicalramp", types.SmallIconButton},
	ClearDressupsCommandID:  {"Dress-up", "cleardressups", types.SmallIconButton},

	// Tool library
	AddEndmillCommandID:  {"Tool Library", "endmill", types.SmallIconButton},
	AddDrillCommandID:    {"Tool Library", "drill", types.SmallIconButton},
	AddBallnoseCommandID: {"Tool Library", "ballnose", types.SmallIconButton},
	RemoveToolCommandID:  {"Tool Library", "removetool", types.SmallIconButton},
	ExportToolsCommandID: {"Tool Library", "exporttools", types.SmallIconButton},
	ImportToolsCommandID: {"Tool Library", "importtools", types.SmallIconButton},
}
