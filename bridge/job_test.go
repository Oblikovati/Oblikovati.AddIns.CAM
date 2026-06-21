// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"strings"
	"testing"
)

// TestStockFromRangeBox covers the cm→mm conversion and the malformed-input guard.
func TestStockFromRangeBox(t *testing.T) {
	s := StockFromRangeBox([]float64{0, 0, 0}, []float64{10, 6, 1}) // cm
	if s.TopZ() != 10 || s.BottomZ() != 0 || s.Max.X != 100 {
		t.Errorf("stock = %+v, want top 10 bottom 0 maxX 100 (mm)", s)
	}
	if zero := StockFromRangeBox([]float64{0}, []float64{1}); zero != (Stock{}) {
		t.Errorf("malformed range box must give zero stock, got %+v", zero)
	}
}

// TestJobGenerateAllSkipsInactive checks inactive operations are skipped and an operation
// error stops the job.
func TestJobGenerateAllSkipsInactive(t *testing.T) {
	j := NewJob()
	j.Tools = []ToolController{{ToolNumber: 1, VertFeed: 100}}
	active := &DrillingOp{OpBase: OpBase{OpLabel: "A", IsActive: true}, Holes: []DrillTarget{{X: 0, Y: 0, Top: 10, Bottom: 0}}}
	inactive := &DrillingOp{OpBase: OpBase{OpLabel: "B", IsActive: false}}
	j.Operations = []Operation{active, inactive}

	results, err := j.GenerateAll()
	if err != nil {
		t.Fatalf("GenerateAll: %v", err)
	}
	if len(results) != 1 || results[0].Label != "A" {
		t.Errorf("want only the active op, got %d results", len(results))
	}

	// An operation that fails (no holes) propagates its error.
	j.Operations = []Operation{&DrillingOp{OpBase: OpBase{OpLabel: "C", IsActive: true}}}
	if _, err := j.GenerateAll(); err == nil {
		t.Error("an operation error must stop the job")
	}
}

// TestToolChangeBlockNoSpindle covers a controller that leaves the spindle off.
func TestToolChangeBlockNoSpindle(t *testing.T) {
	block := toolChangeBlock(ToolController{ToolNumber: 2, SpindleDir: "None"})
	if len(block) != 1 || block[0].Name != "M6" {
		t.Errorf("no-spindle block = %v, want a single M6", commandNames(block))
	}
}

// TestToolChangeBlockSpinUp covers the spindle spin-up dwell: a G4 dwell follows the spindle start
// when a spin-up time is set, is omitted when it is zero, and never appears without a spindle.
func TestToolChangeBlockSpinUp(t *testing.T) {
	withDwell := toolChangeBlock(ToolController{ToolNumber: 1, SpindleSpeed: 8000, SpindleDir: "Forward", SpinUpSecs: 2})
	names := commandNames(withDwell)
	if strings.Join(names, ",") != "M6,M3,G4" {
		t.Errorf("spin-up block = %v, want M6,M3,G4", names)
	}
	if dwell := withDwell[2]; dwell.Params["P"] != 2 {
		t.Errorf("dwell P = %g, want 2 s", dwell.Params["P"])
	}
	noDwell := toolChangeBlock(ToolController{ToolNumber: 1, SpindleSpeed: 8000, SpindleDir: "Forward"})
	if len(noDwell) != 2 {
		t.Errorf("a zero spin-up should add no dwell, got %v", commandNames(noDwell))
	}
	// No spindle → no dwell even with a spin-up time set.
	if b := toolChangeBlock(ToolController{ToolNumber: 1, SpindleDir: "None", SpinUpSecs: 2}); len(b) != 1 {
		t.Errorf("no spindle should mean no dwell, got %v", commandNames(b))
	}
}
