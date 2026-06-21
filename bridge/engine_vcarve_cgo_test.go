// SPDX-License-Identifier: GPL-2.0-only

//go:build cgo

package bridge

import (
	"strings"
	"testing"
)

// TestEngineRunVCarveJob runs the V-carve flow and checks it carves the outline region. V-carve rides
// the medial axis built by the cgo Voronoi engine, so this integration test carries the build tag.
func TestEngineRunVCarveJob(t *testing.T) {
	res, err := NewEngine(&recordingHost{}).SetPost("grbl").RunVCarveJobOnHost(0)
	if err != nil {
		t.Fatalf("RunVCarveJobOnHost: %v", err)
	}
	if !strings.Contains(res.Summary, "v-carved") {
		t.Errorf("summary = %q, want it to mention v-carved", res.Summary)
	}
	if !strings.Contains(res.GCode, "G1") {
		t.Error("V-carve should emit cutting contours")
	}
}
