// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "embed"

// iconFS bundles the CAM ribbon glyphs. Each file is "icons/<key>.svg" in the Oblikovati glyph
// convention (a 24×24 viewBox; the sentinel paints the host recolours per theme: a green fill tile,
// a black outline, and red action accents).
//
//go:embed icons/*.svg
var iconFS embed.FS

// iconSVG returns the inline SVG markup for a CAM button glyph, or "" when no such asset is bundled
// (the host then falls back to a text button). The add-in ships its own glyphs to the host via the
// command IconSVG field (api v0.86.0), so its buttons are not limited to the host's bundled icons.
func iconSVG(key string) string {
	b, err := iconFS.ReadFile("icons/" + key + ".svg")
	if err != nil {
		return ""
	}
	return string(b)
}
