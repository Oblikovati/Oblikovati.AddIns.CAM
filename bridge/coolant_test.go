// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"strings"
	"testing"

	"oblikovati.org/cam/bridge/post"
)

// lastObj returns the final post object — the operation, after the prepended tool-list header.
func lastObj(objs []post.Object) post.Object { return objs[len(objs)-1] }

// TestCoolantBlocks wraps an operation in coolant-on/off when a mode is set.
func TestCoolantBlocks(t *testing.T) {
	res := []OperationResult{{Label: "Profile", Path: NewJobPath("G1 X1"), Coolant: CoolantFlood}}
	names := commandNames(lastObj(PostObjects(res)).Path.Commands)
	joined := strings.Join(names, ",")
	if !strings.Contains(joined, "M8") || !strings.Contains(joined, "M9") {
		t.Errorf("flood coolant should bracket the op with M8…M9, got %v", names)
	}
	// M8 must come before the cut and M9 after it
	if indexOf(names, "M8") > indexOf(names, "G1") || indexOf(names, "M9") < indexOf(names, "G1") {
		t.Errorf("coolant block ordering wrong: %v", names)
	}
}

// TestNoCoolantNoBlocks emits no coolant codes when the mode is none.
func TestNoCoolantNoBlocks(t *testing.T) {
	res := []OperationResult{{Label: "Profile", Path: NewJobPath("G1 X1"), Coolant: CoolantNone}}
	for _, n := range commandNames(lastObj(PostObjects(res)).Path.Commands) {
		if n == "M7" || n == "M8" || n == "M9" {
			t.Errorf("no coolant should emit no M7/M8/M9, got %v", n)
		}
	}
}

// TestCoolantParamEdit edits an operation's coolant through the parameter interface and checks
// it reaches the posted program.
func TestCoolantParamEdit(t *testing.T) {
	op := &ProfileOp{OpBase: OpBase{OpLabel: "P"}}
	if !op.SetParameter("coolant", CoolantMist) || op.CoolantMode() != CoolantMist {
		t.Fatalf("coolant param edit failed: %q", op.Coolant)
	}
	res := []OperationResult{{Label: "P", Path: NewJobPath("G1 X1"), Coolant: coolantOf(op)}}
	if !strings.Contains(strings.Join(commandNames(lastObj(PostObjects(res)).Path.Commands), ","), "M7") {
		t.Error("mist coolant should emit M7")
	}
}

// indexOf returns the first index of s in names, or -1.
func indexOf(names []string, s string) int {
	for i, n := range names {
		if n == s {
			return i
		}
	}
	return -1
}
