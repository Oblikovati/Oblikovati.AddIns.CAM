// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"strconv"
	"strings"

	"oblikovati.org/cam/bridge/post"
)

// Multi-fixture program generation (FreeCAD's Work Coordinate Systems): when the Output tab
// checks several fixtures (G54–G59), the program is posted once per fixture and concatenated.
// The order-by setting decides what repeats per fixture — Fixture repeats the whole program,
// Tool repeats per tool group (minimising tool changes), Operation repeats per operation.

// postProgram posts the operation results to G-code, repeating per selected fixture.
func (e *Engine) postProgram(name string, results []OperationResult) (string, error) {
	e.mu.Lock()
	fixtures := checkedFixtures(e.wcs)
	if len(fixtures) == 0 {
		fixtures = []int{workOffsetOrOne(e.workOffset)}
	}
	order := orderByOrFixture(e.orderBy)
	extra := e.postArguments
	e.mu.Unlock()

	if len(fixtures) == 1 {
		return post.Export(name, PostObjects(results), postArgsFor(fixtures[0], extra))
	}
	var blocks []string
	for _, group := range orderResults(results, order) {
		for _, fixture := range fixtures {
			g, err := post.Export(name, PostObjects(group), postArgsFor(fixture, extra))
			if err != nil {
				return "", err
			}
			blocks = append(blocks, g)
		}
	}
	return strings.Join(blocks, "\n"), nil
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

// orderResults groups the results into the units repeated per fixture: all-as-one for Fixture
// ordering, one group per tool for Tool, one per operation for Operation.
func orderResults(results []OperationResult, order string) [][]OperationResult {
	switch order {
	case "Operation":
		groups := make([][]OperationResult, len(results))
		for i, r := range results {
			groups[i] = []OperationResult{r}
		}
		return groups
	case "Tool":
		return groupResultsByTool(results)
	default: // Fixture
		return [][]OperationResult{results}
	}
}

// groupResultsByTool groups results by tool number, preserving first-seen tool order.
func groupResultsByTool(results []OperationResult) [][]OperationResult {
	var groups [][]OperationResult
	index := map[int]int{}
	for _, r := range results {
		tn := r.Controller.ToolNumber
		gi, ok := index[tn]
		if !ok {
			gi = len(groups)
			index[tn] = gi
			groups = append(groups, nil)
		}
		groups[gi] = append(groups[gi], r)
	}
	return groups
}
