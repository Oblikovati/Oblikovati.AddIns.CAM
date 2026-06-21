// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// feedTC is a controller with round feeds for checking the override.
func feedTC() ToolController {
	return ToolController{ToolNumber: 1, VertFeed: 100, HorizFeed: 300, SpindleSpeed: 5000, SpindleDir: "Forward", Tool: ToolBit{ShapeType: "endmill", Diameter: 6}}
}

// TestFeedFactorDefaults checks an unset FeedScale is full feed (factor 1), and a set one scales.
func TestFeedFactorDefaults(t *testing.T) {
	if f := (&OpBase{}).feedFactor(); f != 1 {
		t.Errorf("an unset feed scale should be full feed (1), got %g", f)
	}
	if f := (&OpBase{FeedScale: 0.4}).feedFactor(); f != 0.4 {
		t.Errorf("feed factor = %g, want 0.4", f)
	}
}

// TestOpBaseFeedsScaled checks the shared feeds builder scales the cutting/plunge feeds but leaves
// the clearance/safe heights alone.
func TestOpBaseFeedsScaled(t *testing.T) {
	b := &OpBase{ClearanceHeight: 15, SafeHeight: 2, FeedScale: 0.5}
	f := b.feeds(feedTC())
	if f.Vert != 50 || f.Horiz != 150 {
		t.Errorf("scaled feeds = vert %g horiz %g, want 50 / 150", f.Vert, f.Horiz)
	}
	if f.ClearanceZ != 15 || f.SafeZ != 2 {
		t.Errorf("heights must not be scaled, got clearance %g safe %g", f.ClearanceZ, f.SafeZ)
	}
}

// TestProfileFeedOverride checks a profile op's cutting moves slow down under a feed override,
// end-to-end through Execute, while an unset scale leaves the tool feed untouched.
func TestProfileFeedOverride(t *testing.T) {
	job := NewJob()
	job.Tools = []ToolController{feedTC()}
	base := func(scale float64) *ProfileOp {
		return &ProfileOp{OpBase: OpBase{OpLabel: "P", IsActive: true, FinalDepth: -2, FeedScale: scale},
			Side: "outside", Boundary: squarePoly(20)}
	}
	full := firstCutFeed(t, base(0), job)
	half := firstCutFeed(t, base(0.5), job)
	if full != 300 {
		t.Errorf("full-feed profile cut F = %g, want the tool's 300", full)
	}
	if half != 150 {
		t.Errorf("half-feed profile cut F = %g, want 150 (0.5×300)", half)
	}
}

// firstCutFeed runs an op and returns the F of its first horizontal G1 cut (a G1 carrying X/Y, so
// the controller's horizontal feed rather than the plunge feed).
func firstCutFeed(t *testing.T, op *ProfileOp, job *Job) float64 {
	t.Helper()
	path, err := op.Execute(job)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, c := range path.Commands {
		_, hasX := c.Params["X"]
		_, hasY := c.Params["Y"]
		if c.Name == "G1" && (hasX || hasY) {
			if f, ok := c.Params["F"]; ok {
				return f
			}
		}
	}
	t.Fatal("no horizontal feed move found")
	return 0
}
