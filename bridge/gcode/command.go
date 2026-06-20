// SPDX-License-Identifier: GPL-2.0-only

// Package gcode is the pure toolpath model — Command (one G/M move) and Path (an ordered
// list of them) — plus the Vector3 the generators consume. It is the leaf of the CAM add-in:
// generators (../gen), operations (..), and post processors (../post) all depend on it, and it
// depends on nothing in this module, mirroring FreeCAD's App/Command + App/Path sitting under
// Path/Base/Generator, Path/Op, and Path/Post.
package gcode

import (
	"sort"
	"strconv"
	"strings"
)

// Vector3 is a point/direction in model space (millimetres in toolpath context). A minimal
// value type so the generators stay free of any kernel geometry dependency.
type Vector3 struct {
	X, Y, Z float64
}

// Command is one toolpath instruction: a G/M-code name plus its addressed parameters
// (X/Y/Z, I/J/K, F, R, Q, P, L, S, T, …). It mirrors FreeCAD's Path.Command — a flat
// (name, parameter-map) pair with no embedded geometry — so a post processor can render
// it to text and a generator can emit it without a kernel dependency. Numeric values are
// kept in millimetres (the G-code convention), independent of the host's centimetre
// database unit; the operation layer converts at the geometry boundary.
type Command struct {
	Name   string             // G/M code, e.g. "G0", "G1", "G81", "M6", or a "(comment)"
	Params map[string]float64 // addressed parameters keyed by single-letter address
}

// NewCommand builds a Command from a name and parameter map. A nil params map is
// normalised to an empty (non-nil) map so callers can index it freely.
func NewCommand(name string, params map[string]float64) Command {
	if params == nil {
		params = map[string]float64{}
	}
	return Command{Name: name, Params: params}
}

// ParseCommand parses a single G-code line ("G0 X10 Y20 Z30 F100") into a Command. A
// line that opens with "(" is treated as a whole-line comment: the entire text becomes
// the Name and no parameters are extracted (matching FreeCAD's Path.Command(string)).
// Unparseable address values are skipped rather than erroring — a post never aborts a
// job on one malformed token.
func ParseCommand(line string) Command {
	s := strings.TrimSpace(line)
	if s == "" {
		return NewCommand("", nil)
	}
	if strings.HasPrefix(s, "(") {
		return NewCommand(s, nil)
	}
	fields := strings.Fields(s)
	cmd := NewCommand(fields[0], nil)
	for _, tok := range fields[1:] {
		if len(tok) < 2 {
			continue
		}
		addr := strings.ToUpper(tok[:1])
		v, err := strconv.ParseFloat(tok[1:], 64)
		if err != nil {
			continue
		}
		cmd.Params[addr] = v
	}
	return cmd
}

// gcodeAddressOrder is the canonical address order for ToGCode's debug rendering. It is
// NOT authoritative for machine output — each post processor (../post) defines its own
// controller-specific ordering and formatting.
var gcodeAddressOrder = []string{"X", "Y", "Z", "A", "B", "C", "I", "J", "K", "F", "S", "T", "Q", "R", "L", "H", "D", "P"}

// ToGCode renders the command in a canonical, deterministic form for logging/debugging.
// Real machine output goes through a post processor; this is a stable string for tests
// and diagnostics only.
func (c Command) ToGCode() string {
	if len(c.Params) == 0 {
		return c.Name
	}
	parts := []string{c.Name}
	seen := map[string]bool{}
	for _, addr := range gcodeAddressOrder {
		if v, ok := c.Params[addr]; ok {
			parts = append(parts, addr+formatNumber(v))
			seen[addr] = true
		}
	}
	// Any address outside the canonical list, appended in sorted order for determinism.
	var extra []string
	for addr := range c.Params {
		if !seen[addr] {
			extra = append(extra, addr)
		}
	}
	sort.Strings(extra)
	for _, addr := range extra {
		parts = append(parts, addr+formatNumber(c.Params[addr]))
	}
	return strings.Join(parts, " ")
}

// formatNumber renders a parameter value without trailing zeros for whole numbers, so
// ToGCode reads like hand-written G-code ("X10" not "X10.000000").
func formatNumber(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// Path is an ordered list of Commands — the toolpath an operation produces and a post
// processor consumes. It mirrors FreeCAD's Path.Path.
type Path struct {
	Commands []Command
}

// NewPath wraps a command slice as a Path.
func NewPath(commands []Command) Path { return Path{Commands: commands} }

// Add appends one command to the path.
func (p *Path) Add(c Command) { p.Commands = append(p.Commands, c) }
