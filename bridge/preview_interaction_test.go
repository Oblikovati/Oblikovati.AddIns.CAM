// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"encoding/json"
	"testing"

	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// previewHost is the contour fake host that also records interaction-graphics calls and the
// lane/nodes of the last update, so a test can assert what the transient preview pushed.
type previewHost struct {
	recordingHost
	lastLane  string
	lastNodes []wire.GraphicsNode
	updates   int
	clears    int
}

func (h *previewHost) Call(method string, req []byte) ([]byte, error) {
	switch method {
	case wire.MethodInteractionGraphicsUpdate:
		var a wire.UpdateInteractionGraphicsArgs
		_ = json.Unmarshal(req, &a)
		h.lastLane, h.lastNodes = a.Lane, a.Nodes
		h.updates++
		return []byte("{}"), nil
	case wire.MethodInteractionGraphicsClear:
		h.clears++
		return []byte("{}"), nil
	}
	return h.recordingHost.Call(method, req)
}

// TestPreviewProfilePushesTransientLane previews the profile and checks it went to the
// interaction OVERLAY lane as a single node — not the persistent client-graphics set — and
// that no G-code was posted.
func TestPreviewProfilePushesTransientLane(t *testing.T) {
	h := &previewHost{}
	res, err := NewEngine(h).PreviewProfileOnHost(0)
	if err != nil {
		t.Fatalf("PreviewProfileOnHost: %v", err)
	}
	if h.updates != 1 {
		t.Fatalf("interaction update count = %d, want 1", h.updates)
	}
	if h.lastLane != string(types.GraphicsLaneOverlay) {
		t.Errorf("preview lane = %q, want %q", h.lastLane, types.GraphicsLaneOverlay)
	}
	if len(h.lastNodes) != 1 || h.lastNodes[0].Id != PreviewNodeID {
		t.Errorf("preview nodes = %+v, want one node id %q", h.lastNodes, PreviewNodeID)
	}
	if len(h.lastNodes[0].Primitives) == 0 {
		t.Error("preview node carried no line primitives")
	}
	if res.GCode != "" {
		t.Errorf("preview must not post G-code, got %d bytes", len(res.GCode))
	}
	if h.called(wire.MethodClientGraphicsSet) {
		t.Error("preview must not touch the persistent client-graphics overlay")
	}
}

// TestCommitClearsPreview confirms committing a profile job clears the transient preview so
// the persistent overlay takes over.
func TestCommitClearsPreview(t *testing.T) {
	h := &previewHost{}
	if _, err := NewEngine(h).RunProfileJobOnHost(0); err != nil {
		t.Fatalf("RunProfileJobOnHost: %v", err)
	}
	if h.clears == 0 {
		t.Error("committing a job must clear the transient toolpath preview")
	}
}

// TestClearPreviewAction covers the Clear-Preview command handler.
func TestClearPreviewAction(t *testing.T) {
	h := &previewHost{}
	res, err := NewEngine(h).clearPreviewAction()
	if err != nil {
		t.Fatalf("clearPreviewAction: %v", err)
	}
	if h.clears != 1 || res.Summary == "" {
		t.Errorf("clear-preview action: clears=%d summary=%q", h.clears, res.Summary)
	}
}
