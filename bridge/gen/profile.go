// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"errors"
	"fmt"

	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/geom2d"
)

// Profile side constants: which side of the boundary the tool runs on, accounting for tool
// radius. Outside leaves the boundary as the finished wall (cut grows out by the radius);
// inside cuts a slot/hole to the boundary (cut shrinks in); on runs the tool centre on the
// boundary (no compensation).
const (
	SideOutside = "outside"
	SideInside  = "inside"
	SideOn      = "on"
)

// ProfileParams configure a contour (profile) pass.
type ProfileParams struct {
	ToolRadius  float64 // mm
	Side        string  // SideOutside | SideInside | SideOn
	OffsetExtra float64 // extra stock left on the wall (mm), added to the radius offset
	Climb       bool    // climb (CCW outside) vs conventional milling
}

// GenerateProfile cuts a contour around the boundary at each depth level: the boundary is
// radius-compensated to the chosen side, oriented for the cut direction, and walked at every
// Z. Ports the toolpath shape of FreeCAD's Path/Op/Profile (the offset + Z-stepdown core).
func GenerateProfile(boundary geom2d.Polygon, levels []float64, feeds Feeds, p ProfileParams) ([]gcode.Command, error) {
	if p.ToolRadius <= 0 {
		return nil, fmt.Errorf("profile needs a positive tool radius, got %g", p.ToolRadius)
	}
	if len(boundary) < 3 {
		return nil, errors.New("profile boundary needs at least 3 points")
	}
	path, err := compensate(boundary, p)
	if err != nil {
		return nil, err
	}
	path = orient(path, p.Climb)

	var cmds []gcode.Command
	for _, z := range levels {
		cmds = append(cmds, walkLoop(path, z, feeds)...)
	}
	return cmds, nil
}

// compensate offsets the boundary to the requested side by (radius + extra). The "on" side
// runs the tool centre on the boundary (no offset). An inside offset that collapses means
// the tool is too large for the feature.
func compensate(boundary geom2d.Polygon, p ProfileParams) (geom2d.Polygon, error) {
	if p.Side == SideOn {
		return boundary.EnsureCCW(), nil
	}
	d := p.ToolRadius + p.OffsetExtra
	if p.Side == SideInside {
		d = -d
	}
	path, ok := geom2d.Offset(boundary, d)
	if !ok {
		return nil, fmt.Errorf("profile side %q: tool radius %g (+%g extra) collapses the contour — tool too large for the feature",
			p.Side, p.ToolRadius, p.OffsetExtra)
	}
	return path, nil
}
