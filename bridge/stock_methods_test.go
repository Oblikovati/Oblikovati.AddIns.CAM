// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/api/wire"
)

// TestBoxStockExplicitDims checks an explicit box is centred on the model XY and rises from its
// bottom (model range box [0,0,0]–[1,1,0.5] cm → [0,0,0]–[10,10,5] mm; box 20×30×8 mm).
func TestBoxStockExplicitDims(t *testing.T) {
	s := boxStock([]float64{0, 0, 0}, []float64{1, 1, 0.5}, 20, 30, 8)
	if s.Min.X != -5 || s.Max.X != 15 || s.Min.Y != -10 || s.Max.Y != 20 || s.Min.Z != 0 || s.Max.Z != 8 {
		t.Errorf("box stock = %+v", s)
	}
}

// TestBoxStockDefaultsToModel checks zero dimensions fall back to the model bounding-box size.
func TestBoxStockDefaultsToModel(t *testing.T) {
	s := boxStock([]float64{0, 0, 0}, []float64{1, 1.2, 0.5}, 0, 0, 0)
	if s.Max.X-s.Min.X != 10 || s.Max.Y-s.Min.Y != 12 || s.Max.Z-s.Min.Z != 5 {
		t.Errorf("default box = %+v", s)
	}
}

// TestCylinderStockBbox checks the cylinder's bounding box (±radius in XY, height in Z).
func TestCylinderStockBbox(t *testing.T) {
	s := cylinderStock([]float64{0, 0, 0}, []float64{1, 1, 0.5}, 8, 6)
	if s.Min.X != -3 || s.Max.X != 13 || s.Min.Z != 0 || s.Max.Z != 6 {
		t.Errorf("cylinder stock = %+v", s)
	}
}

// TestStockForUsesMethod checks the engine builds the chosen stock method (box height applied).
func TestStockForUsesMethod(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.stockMethod = StockBox
	e.stockBoxL, e.stockBoxW, e.stockBoxH = 100, 100, 20
	s := e.stockFor([]float64{0, 0, 0}, []float64{1, 1, 0.5})
	if s.Max.Z-s.Min.Z != 20 {
		t.Errorf("box-method height = %v, want 20", s.Max.Z-s.Min.Z)
	}
}

// TestSetupTabSwapsStockFields checks the Setup tab shows the method dropdown and the chosen
// method's fields (cylinder radius/height).
func TestSetupTabSwapsStockFields(t *testing.T) {
	h := &recordingHost{}
	e := NewEngine(h)
	e.lastJob = &Job{}
	e.stockMethod = StockCylinder
	if _, err := e.ShowJobEditWindow(); err != nil {
		t.Fatalf("ShowJobEditWindow: %v", err)
	}
	win := h.dockWindows[len(h.dockWindows)-1]
	for _, id := range []string{"stock_method", "stock_cyl_r", "stock_cyl_h"} {
		if _, ok := findControl(win.Controls, func(c wire.PanelControlSpec) bool { return c.ID == id }); !ok {
			t.Errorf("Setup tab missing %q for the cylinder method", id)
		}
	}
	if _, ok := findControl(win.Controls, func(c wire.PanelControlSpec) bool { return c.ID == "stock_box_l" }); ok {
		t.Error("box fields should not show for the cylinder method")
	}
}
