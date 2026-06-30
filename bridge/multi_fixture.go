// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"strconv"
	"strings"

	"oblikovati.org/cam/bridge/post"
)

// Multi-fixture program generation (FreeCAD's Work Coordinate Systems): when the Output tab
// checks several fixtures (G54–G59), the program is built as one unit per the order-by setting —
// Fixture (a unit per fixture, the whole program), Tool (a unit per tool group), or Operation (a
// unit per operation). The units are concatenated into one program, or, with split output on,
// written to a separate .nc file each.

// namedProgram is one posted output unit: its file-name suffix and its G-code.
type namedProgram struct {
	Suffix string
	GCode  string
}

// postProgram posts the operation results, honouring the selected fixtures and order-by. It
// returns the concatenated program (for display/save) and, when split output is on and there is
// more than one unit, stores the units for a per-file save.
func (e *Engine) postProgram(name string, results []OperationResult) (string, error) {
	e.mu.Lock()
	fixtures := checkedFixtures(e.wcs)
	if len(fixtures) == 0 {
		fixtures = []int{workOffsetOrOne(e.workOffset)}
	}
	order := orderByOrFixture(e.orderBy)
	extra := e.postArguments
	split := e.splitOutput
	e.mu.Unlock()

	programs, err := buildPrograms(name, results, fixtures, order, extra)
	if err != nil {
		return "", err
	}
	e.setLastPrograms(splitUnits(split, programs))
	return concatPrograms(programs), nil
}

// buildPrograms produces the output units for the order-by mode: a unit per fixture (Fixture), a
// unit per tool group across all fixtures (Tool), or a unit per operation across all fixtures
// (Operation). The Fixture mode posts each fixture's whole program in one pass, so the default
// single-fixture output is unchanged.
func buildPrograms(name string, results []OperationResult, fixtures []int, order, extra string) ([]namedProgram, error) {
	switch order {
	case "Operation":
		return unitsAcrossFixtures(name, perOperation(results), fixtures, extra)
	case "Tool":
		return unitsAcrossFixtures(name, groupResultsByTool(results), fixtures, extra)
	default: // Fixture
		return fixtureUnits(name, results, fixtures, extra)
	}
}

// fixtureUnits posts one unit per fixture, each the whole program at that fixture's offset.
func fixtureUnits(name string, results []OperationResult, fixtures []int, extra string) ([]namedProgram, error) {
	programs := make([]namedProgram, 0, len(fixtures))
	for _, fixture := range fixtures {
		g, err := post.Export(name, PostObjects(results), postArgsFor(fixture, extra))
		if err != nil {
			return nil, err
		}
		programs = append(programs, namedProgram{Suffix: fixtureSuffix(fixture), GCode: g})
	}
	return programs, nil
}

// unitsAcrossFixtures posts one unit per group, each spanning every fixture (so a tool/operation
// unit gathers its work across all fixtures into one program).
func unitsAcrossFixtures(name string, groups []resultGroup, fixtures []int, extra string) ([]namedProgram, error) {
	programs := make([]namedProgram, 0, len(groups))
	for _, group := range groups {
		var blocks []string
		for _, fixture := range fixtures {
			g, err := post.Export(name, PostObjects(group.results), postArgsFor(fixture, extra))
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, g)
		}
		programs = append(programs, namedProgram{Suffix: group.suffix, GCode: strings.Join(blocks, "\n")})
	}
	return programs, nil
}

// resultGroup is a named subset of operation results (a tool group or a single operation).
type resultGroup struct {
	suffix  string
	results []OperationResult
}

// perOperation makes one group per operation.
func perOperation(results []OperationResult) []resultGroup {
	groups := make([]resultGroup, len(results))
	for i, r := range results {
		groups[i] = resultGroup{suffix: operationSuffix(i, r), results: []OperationResult{r}}
	}
	return groups
}

// groupResultsByTool groups results by tool number, preserving first-seen tool order.
func groupResultsByTool(results []OperationResult) []resultGroup {
	var groups []resultGroup
	index := map[int]int{}
	for _, r := range results {
		tn := r.Controller.ToolNumber
		gi, ok := index[tn]
		if !ok {
			gi = len(groups)
			index[tn] = gi
			groups = append(groups, resultGroup{suffix: fmt.Sprintf("T%d", tn)})
		}
		groups[gi].results = append(groups[gi].results, r)
	}
	return groups
}

// postArgsFor builds the post argument string for one fixture: the GUI no-op, the fixture's
// --work-offset (G54 is implicit, so left off), and any extra user arguments.
func postArgsFor(fixture int, extra string) string {
	args := "--no-show-editor"
	if fixture >= 2 && fixture <= 6 {
		args += " --work-offset=G5" + strconv.Itoa(3+fixture) // 2→G55 … 6→G59
	}
	if extra = strings.TrimSpace(extra); extra != "" {
		args += " " + extra
	}
	return args
}

// checkedFixtures returns the selected work coordinate systems (1..6 → G54..G59), ascending.
func checkedFixtures(wcs map[int]bool) []int {
	var out []int
	for n := 1; n <= 6; n++ {
		if wcs[n] {
			out = append(out, n)
		}
	}
	return out
}

// fixtureSuffix names a fixture file: 1→G54 … 6→G59.
func fixtureSuffix(fixture int) string { return fmt.Sprintf("G5%d", 3+fixture) }

// operationSuffix names an operation file by its label, or its 1-based index.
func operationSuffix(i int, r OperationResult) string {
	if s := sanitizeSuffix(r.Label); s != "" {
		return s
	}
	return fmt.Sprintf("op%d", i+1)
}

// sanitizeSuffix keeps a label safe for a file name (alphanumerics and dashes; spaces → _).
func sanitizeSuffix(label string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(label) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		case r == ' ' || r == '_':
			b.WriteByte('_')
		}
	}
	return b.String()
}

// concatPrograms joins the units into one program (the non-split output).
func concatPrograms(programs []namedProgram) string {
	parts := make([]string, len(programs))
	for i, p := range programs {
		parts[i] = p.GCode
	}
	return strings.Join(parts, "\n")
}

// splitUnits returns the units to save separately — only when split output is on and there is more
// than one (a single unit always saves as one file).
func splitUnits(split bool, programs []namedProgram) []namedProgram {
	if split && len(programs) > 1 {
		return programs
	}
	return nil
}

// setLastPrograms records the split output units for the next save (nil = save as one file).
func (e *Engine) setLastPrograms(programs []namedProgram) {
	e.mu.Lock()
	e.lastPrograms = programs
	e.mu.Unlock()
}
