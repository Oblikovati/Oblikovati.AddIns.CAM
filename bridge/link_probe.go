// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"oblikovati.org/api/client"
	"oblikovati.org/api/wire"
	"oblikovati.org/cam/bridge/gcode"
	"oblikovati.org/cam/bridge/link"
)

// dbUnitMM is the host's database length unit in millimetres: the kernel works in centimetres, so a
// toolpath millimetre is 0.1 database unit. Coordinates and distances cross the host boundary by
// this factor.
const dbUnitMM = 10.0

// partProbe answers link.CollisionProbe over the host's body.minimumDistance for one body — the
// out-of-process collision query keep-tool-down linking needs. Toolpath lengths are millimetres and
// the host works in centimetres, so coordinates and the result are scaled by dbUnitMM at the call.
type partProbe struct {
	api       *client.Client
	bodyIndex int
}

// PartClearance returns the minimum distance (mm) from the part to the travel polyline, the probe
// widened by the tool radius. It projects the host body.minimumDistance query.
func (p partProbe) PartClearance(pts []gcode.Vector3, toolRadius float64) (float64, error) {
	coords := make([]float64, 0, len(pts)*3)
	for _, q := range pts {
		coords = append(coords, q.X/dbUnitMM, q.Y/dbUnitMM, q.Z/dbUnitMM)
	}
	res, err := p.api.Body().MinimumDistance(wire.MinimumDistanceArgs{
		BodyIndex: p.bodyIndex,
		Points:    coords,
		Radius:    toolRadius / dbUnitMM,
	})
	if err != nil {
		return 0, err
	}
	return res.Distance * dbUnitMM, nil
}

// applyKeepToolDown lowers each operation's between-cut retracts to the lowest plane that clears the
// part, querying the host for collision. The part is the job's first model body; with no model body
// (nothing to collide with) the toolpaths are returned as generated. Best-effort per op: a host
// error leaves that op's path unchanged, so a link query never fails a whole program.
func (e *Engine) applyKeepToolDown(job *Job, results []OperationResult) []OperationResult {
	if len(job.ModelBodies) == 0 {
		return results
	}
	probe := partProbe{api: e.api, bodyIndex: job.ModelBodies[0]}
	for i := range results {
		toolRadius := results[i].Controller.Tool.Diameter / 2
		optimized, err := link.OptimizeRetracts(
			results[i].Path, results[i].SafeZ, results[i].ClearanceZ, probe, toolRadius, link.DefaultClearance)
		if err != nil {
			continue
		}
		results[i].Path = optimized
	}
	return results
}
