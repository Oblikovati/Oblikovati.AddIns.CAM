// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"fmt"
	"math"

	"oblikovati.org/cam/bridge/gcode"
)

// CountersinkParams configure a countersink: a conical recess cut at a hole top so a flat-head
// screw sits flush. The tool spirals inward and down from the sink rim (widest, at the surface)
// to the centre, tracing the cone; the cone's depth follows the tool's included angle so the rim
// reaches Diameter at the surface.
type CountersinkParams struct {
	Diameter     float64 // mm — countersink rim diameter at the surface
	ToolAngleDeg float64 // included angle of the countersink/V-tool (degrees); <=0 → 90°
	ToolDiameter float64 // mm — sets the spiral's radial pitch
	StepOver     float64 // fraction of tool diameter between spiral turns (0..1); 0 → 0.5
}

// segmentsPerTurn is how many straight segments approximate one turn of the conical spiral.
const segmentsPerTurn = 24

// GenerateCountersink cuts a conical countersink at the centre point: an inward-and-down spiral
// from the rim radius at the surface to the centre at the cone depth (Diameter / 2 /
// tan(halfAngle)). Emitted as short feed moves so the cone wall is covered at the step-over.
func GenerateCountersink(center gcode.Vector3, feed float64, p CountersinkParams) ([]gcode.Command, error) {
	if p.ToolDiameter <= 0 {
		return nil, fmt.Errorf("countersink needs a positive tool diameter, got %g", p.ToolDiameter)
	}
	if p.Diameter <= 0 {
		return nil, fmt.Errorf("countersink needs a positive diameter, got %g", p.Diameter)
	}
	sinkR := p.Diameter / 2
	half := chamferHalfAngle(p.ToolAngleDeg) // reused from the chamfer generator (defaults to 45°)
	depth := sinkR / math.Tan(half)
	turns := countersinkTurns(sinkR, p)

	cmds := []gcode.Command{
		gcode.NewCommand("G0", map[string]float64{"X": center.X + sinkR, "Y": center.Y}),
		gcode.NewCommand("G1", map[string]float64{"Z": center.Z, "F": feed}),
	}
	segs := turns * segmentsPerTurn
	for i := 1; i <= segs; i++ {
		t := float64(i) / float64(segs)
		ang := t * float64(turns) * 2 * math.Pi
		r := sinkR * (1 - t)
		cmds = append(cmds, gcode.NewCommand("G1", map[string]float64{
			"X": center.X + r*math.Cos(ang),
			"Y": center.Y + r*math.Sin(ang),
			"Z": center.Z - depth*t,
			"F": feed,
		}))
	}
	return cmds, nil
}

// countersinkTurns is the number of spiral turns to cover the rim radius at the step-over, at
// least one.
func countersinkTurns(sinkR float64, p CountersinkParams) int {
	frac := p.StepOver
	if frac <= 0 {
		frac = 0.5
	}
	turns := int(math.Ceil(sinkR / (frac * p.ToolDiameter)))
	if turns < 1 {
		return 1
	}
	return turns
}
