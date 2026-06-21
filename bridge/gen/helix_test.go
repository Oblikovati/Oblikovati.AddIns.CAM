// SPDX-License-Identifier: GPL-2.0-only

package gen

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"oblikovati.org/cam/bridge/gcode"
)

// referenceGCode renders a command as the name followed by its parameters in alphabetical
// address order, each formatted %.6f — so the generator can be compared byte-for-byte against
// the reference oracle string.
func referenceGCode(c gcode.Command) string {
	addrs := make([]string, 0, len(c.Params))
	for a := range c.Params {
		addrs = append(addrs, a)
	}
	sort.Strings(addrs)
	parts := []string{c.Name}
	for _, a := range addrs {
		parts = append(parts, fmt.Sprintf("%s%.6f", a, c.Params[a]))
	}
	return strings.Join(parts, " ")
}

// expectedHelixGCode is the reference oracle string — the concatenated G-code of the annulus
// helix for the canonical args below.
const expectedHelixGCode = "G0 Z23.000000" +
	"G0 X7.500000 Y5.000000" +
	"G1 Z20.000000" +
	"G2 I-2.500000 J0.000000 X2.500000 Y5.000000 Z19.666667" +
	"G2 I2.500000 J0.000000 X7.500000 Y5.000000 Z19.333333" +
	"G2 I-2.500000 J0.000000 X2.500000 Y5.000000 Z19.000000" +
	"G2 I2.500000 J0.000000 X7.500000 Y5.000000 Z18.666667" +
	"G2 I-2.500000 J0.000000 X2.500000 Y5.000000 Z18.333333" +
	"G2 I2.500000 J0.000000 X7.500000 Y5.000000 Z18.000000" +
	"G2 I-2.500000 J0.000000 X2.500000 Y5.000000 Z18.000000" +
	"G2 I2.500000 J0.000000 X7.500000 Y5.000000 Z18.000000" +
	"G0 X6.250000 Y5.000000 Z23.000000" +
	"G0 X10.000000 Y5.000000" +
	"G1 Z20.000000" +
	"G2 I-5.000000 J0.000000 X0.000000 Y5.000000 Z19.500000" +
	"G2 I5.000000 J0.000000 X10.000000 Y5.000000 Z19.000000" +
	"G2 I-5.000000 J0.000000 X0.000000 Y5.000000 Z18.500000" +
	"G2 I5.000000 J0.000000 X10.000000 Y5.000000 Z18.000000" +
	"G2 I-5.000000 J0.000000 X0.000000 Y5.000000 Z18.000000" +
	"G2 I5.000000 J0.000000 X10.000000 Y5.000000 Z18.000000" +
	"G0 X8.750000 Y5.000000 Z23.000000" +
	"G0 X12.500000 Y5.000000" +
	"G1 Z20.000000" +
	"G2 I-7.500000 J0.000000 X-2.500000 Y5.000000 Z19.500000" +
	"G2 I7.500000 J0.000000 X12.500000 Y5.000000 Z19.000000" +
	"G2 I-7.500000 J0.000000 X-2.500000 Y5.000000 Z18.500000" +
	"G2 I7.500000 J0.000000 X12.500000 Y5.000000 Z18.000000" +
	"G2 I-7.500000 J0.000000 X-2.500000 Y5.000000 Z18.000000" +
	"G2 I7.500000 J0.000000 X12.500000 Y5.000000 Z18.000000" +
	"G0 X11.250000 Y5.000000 Z23.000000"

// TestHelixGeneratorOracle checks the annulus helix (outer 7.5, inner 2.5, step 2.5, tool ⌀5,
// ramp 3°, CW, from Inside) over the (5,5,20)→(5,5,18) edge matches the reference G-code exactly.
func TestHelixGeneratorOracle(t *testing.T) {
	cmds, err := GenerateHelix(
		gcode.Vector3{X: 5, Y: 5, Z: 20}, gcode.Vector3{X: 5, Y: 5, Z: 18},
		HelixParams{
			OuterRadius: 7.5, InnerRadius: 2.5, Pitch: 1.0, Step: 2.5, ToolDiameter: 5.0,
			RetractHeight: 23, Direction: HelixCW, StartAt: StartInside, FinishCircle: true,
			RampAngleRad: 3 * 3.141592653589793 / 180,
		})
	if err != nil {
		t.Fatalf("GenerateHelix: %v", err)
	}
	var b strings.Builder
	for _, c := range cmds {
		b.WriteString(referenceGCode(c))
	}
	if got := b.String(); got != expectedHelixGCode {
		t.Errorf("helix g-code mismatch:\n got %q\nwant %q", got, expectedHelixGCode)
	}
}

// TestHelixGuards covers the geometry/parameter guards.
func TestHelixGuards(t *testing.T) {
	top, bottom := gcode.Vector3{X: 0, Y: 0, Z: 10}, gcode.Vector3{X: 0, Y: 0, Z: 0}
	base := HelixParams{OuterRadius: 5, Pitch: 1, RampAngleRad: 1.2, Direction: HelixCW}
	if _, err := GenerateHelix(gcode.Vector3{X: 1, Y: 0, Z: 10}, bottom, base); err == nil {
		t.Error("non-Z-aligned edge must error")
	}
	bad := base
	bad.OuterRadius = 0
	if _, err := GenerateHelix(top, bottom, bad); err == nil {
		t.Error("zero outer radius must error")
	}
	bad = base
	bad.Pitch = 0
	if _, err := GenerateHelix(top, bottom, bad); err == nil {
		t.Error("zero pitch must error")
	}
}
