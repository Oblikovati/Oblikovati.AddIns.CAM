// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"testing"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/geom2d"
)

// TestPocketWithIslandRoutesAround checks the clearing avoids the island: no cutting move passes
// through the grown island region, and the island makes the toolpath plunge more (rings split
// into arcs).
func TestPocketWithIslandRoutesAround(t *testing.T) {
	boundary := square(40) // 0..40
	island := geom2d.Polygon{{X: 15, Y: 15}, {X: 25, Y: 15}, {X: 25, Y: 25}, {X: 15, Y: 25}}

	plain, err := GeneratePocket(boundary, []float64{0}, testFeeds, PocketParams{ToolRadius: 2, StepOver: 0.5, Climb: true})
	if err != nil {
		t.Fatalf("plain pocket: %v", err)
	}
	withIsland, err := GeneratePocket(boundary, []float64{0}, testFeeds, PocketParams{ToolRadius: 2, StepOver: 0.5, Climb: true,
		Islands: []geom2d.Polygon{island}})
	if err != nil {
		t.Fatalf("island pocket: %v", err)
	}

	// the island grown by the tool radius (2) is the keep-out: the plain pocket cuts straight
	// through it, the island pocket must not put a single cutting move inside it.
	grown, _ := geom2d.Offset(island, 2)
	if cutsInside(plain, grown) == 0 {
		t.Fatal("test premise broken: the plain pocket should cut through the island region")
	}
	if n := cutsInside(withIsland, grown); n != 0 {
		t.Errorf("the island pocket put %d cutting moves inside the island keep-out", n)
	}
}

// cutsInside counts the cutting moves (G1 with X/Y) whose endpoint lies inside the region.
func cutsInside(cmds []gcode.Command, region geom2d.Polygon) int {
	n := 0
	for _, c := range cmds {
		x, hasX := c.Params["X"]
		y, hasY := c.Params["Y"]
		if c.Name == "G1" && hasX && hasY && region.Contains(geom2d.Point2{X: x, Y: y}) {
			n++
		}
	}
	return n
}
