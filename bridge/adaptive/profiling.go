// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

// profiling.go ports Adaptive2d::Execute step 4: the profiling op types. Instead of clearing a
// whole pocket, profiling rides a band along each profile curve. Step 4 turns every input profile
// (a closed curve tagged Z=1) into that band area — the region between the profile and the profile
// offset by 2–3 tool diameters — tagging the far offset edge Z=0 so only the real profile wall is
// finished. The band then flows through the same region loop the clearing ops use.

package adaptive

import "oblikovati.org/cam/bridge/clipper"

// profileToAreas turns each input profile curve into a band area between the profile and its offset,
// accumulating their union. Exact port of Execute step 4: the offset is by 2*(toolRadius +
// helixRampMaxRadius + finishOffset) + one min step (negative for inside profiling, so the band sits
// inside the curve), the offset edge is tagged Z=0 (the far wall, never finished), and the
// orientation flips mirror the upstream so the union forms a ring rather than cancelling.
func (s *solver) profileToAreas(inputPaths clipper.Paths) (clipper.Paths, error) {
	offset := float64(2*(s.toolRadiusScaled+s.helixRampMaxRadiusScaled+s.finishPassOffsetScaled)) + minStepClipper
	if s.cfg.OpType == ProfilingInside {
		offset = -offset
	}

	var fullPaths clipper.Paths
	for _, path := range inputPaths {
		offsetPaths, err := clipper.Offset(clipper.Paths{path}, clipper.Round, clipper.ClosedPolygon, offset, 0, 0)
		if err != nil {
			return nil, err
		}
		// Orient the profile wall (Z=1) for the ring union.
		if clipper.Orientation(path) != (offset > 0) {
			clipper.ReversePath(path)
		}
		clip := clipper.Paths{path}
		for _, op := range offsetPaths {
			tagZ(clipper.Paths{op}, 0) // the far offset edge is a stock boundary, not a wall to finish
			if clipper.Orientation(op) != (offset < 0) {
				clipper.ReversePath(op)
			}
			clip = append(clip, op)
		}
		fullPaths, err = clipper.Boolean(clipper.Union, clipper.EvenOdd, fullPaths, true, clip, false)
		if err != nil {
			return nil, err
		}
	}
	return fullPaths, nil
}
