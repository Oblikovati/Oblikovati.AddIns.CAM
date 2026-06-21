// SPDX-License-Identifier: GPL-2.0-only

package gcode

import "testing"

// safePath retracts to clearance (15) before every lateral rapid — the well-formed shape.
func safePath() Path {
	return Path{Commands: []Command{
		NewCommand("G0", map[string]float64{"Z": 15}),
		NewCommand("G0", map[string]float64{"X": 0, "Y": 0}),
		NewCommand("G1", map[string]float64{"Z": -2, "F": 100}),
		NewCommand("G1", map[string]float64{"X": 5, "Y": 0, "F": 200}),
		NewCommand("G0", map[string]float64{"Z": 15}), // retract before crossing
		NewCommand("G0", map[string]float64{"X": 20, "Y": 20}),
	}}
}

// TestLintRapidsAcceptsSafePath finds nothing wrong with a path that always retracts first.
func TestLintRapidsAcceptsSafePath(t *testing.T) {
	if w := LintRapids(safePath()); len(w) != 0 {
		t.Errorf("a safe path should produce no warnings, got %v", w)
	}
}

// TestLintRapidsFlagsRapidThroughStock flags a lateral rapid taken below clearance without a
// retract — the dangerous move.
func TestLintRapidsFlagsRapidThroughStock(t *testing.T) {
	bad := Path{Commands: []Command{
		NewCommand("G0", map[string]float64{"Z": 15}),
		NewCommand("G0", map[string]float64{"X": 0, "Y": 0}),
		NewCommand("G1", map[string]float64{"Z": -2, "F": 100}),
		NewCommand("G1", map[string]float64{"X": 5, "Y": 0, "F": 200}),
		NewCommand("G0", map[string]float64{"X": 20, "Y": 20}), // rapid across still at Z=-2!
	}}
	w := LintRapids(bad)
	if len(w) != 1 {
		t.Fatalf("expected one warning for the rapid through stock, got %d: %v", len(w), w)
	}
}

// TestLintRapidsIgnoresInitialPositioning does not flag the first lateral rapid, made before any
// retract has established a clearance plane (the tool is at a safe machine height).
func TestLintRapidsIgnoresInitialPositioning(t *testing.T) {
	p := Path{Commands: []Command{
		NewCommand("G0", map[string]float64{"X": 3, "Y": 4}), // initial positioning, no plane yet
		NewCommand("G0", map[string]float64{"Z": 15}),
		NewCommand("G1", map[string]float64{"Z": -1, "F": 100}),
	}}
	if w := LintRapids(p); len(w) != 0 {
		t.Errorf("initial positioning should not be flagged, got %v", w)
	}
}
