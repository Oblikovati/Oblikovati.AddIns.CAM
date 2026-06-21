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
	ToolRadius     float64 // mm
	Side           string  // SideOutside | SideInside | SideOn
	OffsetExtra    float64 // extra stock left on the wall (mm), added to the radius offset
	Climb          bool    // climb (CCW outside) vs conventional milling
	RoughingPasses int     // number of radial passes to reach the wall (>1 roughs thick stock); 0/1 → single pass
	RoughStep      float64 // radial step between roughing passes (mm); only used when RoughingPasses > 1
}

// maxProfilePasses caps the roughing passes so a tiny step can't explode the path.
const maxProfilePasses = 100

// GenerateProfile cuts a contour around the boundary at each depth level: the boundary is
// radius-compensated to the chosen side, oriented for the cut direction, and walked at every
// Z. Ports the toolpath shape of FreeCAD's Path/Op/Profile (the offset + Z-stepdown core).
//
// With RoughingPasses > 1 it takes several radial passes per level, from the outermost (one
// RoughStep × (passes−1) of extra stock away from the wall) inward to the final contour — for
// cutting a part out of thick solid stock where a single full-width pass would overload the tool.
func GenerateProfile(boundary geom2d.Polygon, levels []float64, feeds Feeds, p ProfileParams) ([]gcode.Command, error) {
	if p.ToolRadius <= 0 {
		return nil, fmt.Errorf("profile needs a positive tool radius, got %g", p.ToolRadius)
	}
	if len(boundary) < 3 {
		return nil, errors.New("profile boundary needs at least 3 points")
	}
	// The final (at-the-wall) pass must be feasible; an infeasible roughing pass is skipped below.
	if _, err := compensate(boundary, p); err != nil {
		return nil, err
	}

	extras := profileExtras(p)
	var cmds []gcode.Command
	for _, z := range levels {
		for _, extra := range extras {
			pass := p
			pass.OffsetExtra = extra
			path, err := compensate(boundary, pass)
			if err != nil {
				continue // a roughing pass that collapses (inside, too small) is simply skipped
			}
			cmds = append(cmds, walkLoop(orient(path, p.Climb), z, feeds)...)
		}
	}
	return cmds, nil
}

// profileExtras returns the per-pass wall offsets, outermost first and the final wall last: just
// the base offset for a single pass, or the roughing ladder when RoughingPasses > 1. Multi-pass
// makes no sense for the un-compensated "on" side, which always returns the single base offset.
func profileExtras(p ProfileParams) []float64 {
	if p.Side == SideOn || p.RoughingPasses <= 1 || p.RoughStep <= 0 {
		return []float64{p.OffsetExtra}
	}
	n := p.RoughingPasses
	if n > maxProfilePasses {
		n = maxProfilePasses
	}
	extras := make([]float64, 0, n)
	for j := n - 1; j >= 0; j-- {
		extras = append(extras, p.OffsetExtra+float64(j)*p.RoughStep)
	}
	return extras
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
