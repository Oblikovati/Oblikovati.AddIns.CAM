// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"oblikovati.org/api/wire"
	"oblikovati.org/cam/bridge/gcode"
)

// Stock creation methods (FreeCAD's Setup-tab Stock): extend the model's bounding box by margins
// (the default), an explicit box, an explicit cylinder, or an existing solid in the document. The
// Stock value stays an axis-aligned mm box (operations frame depths from its top/bottom and clear
// to its extent); a cylinder is represented by its bounding box.

const (
	StockExtend   = "Extend bbox"
	StockBox      = "Box"
	StockCylinder = "Cylinder"
	StockExisting = "Existing"
)

// stockMethodOptions are the stock creation methods offered in the Setup tab.
func stockMethodOptions() []string {
	return []string{StockExtend, StockBox, StockCylinder, StockExisting}
}

// stockMethodOrExtend defaults an unset/unknown method to extend-bounding-box.
func stockMethodOrExtend(method string) string {
	switch method {
	case StockBox, StockCylinder, StockExisting:
		return method
	}
	return StockExtend
}

// boxStock builds an explicit box centred on the model's XY, rising from the model's bottom. A
// zero dimension falls back to the model's bounding-box size on that axis. min/max are the model
// range box in centimetres.
func boxStock(min, max []float64, length, width, height float64) Stock {
	model := StockFromRangeBox(min, max)
	if length <= 0 {
		length = model.Max.X - model.Min.X
	}
	if width <= 0 {
		width = model.Max.Y - model.Min.Y
	}
	if height <= 0 {
		height = model.Max.Z - model.Min.Z
	}
	cx, cy := (model.Min.X+model.Max.X)/2, (model.Min.Y+model.Max.Y)/2
	return Stock{
		Min: gcode.Vector3{X: cx - length/2, Y: cy - width/2, Z: model.Min.Z},
		Max: gcode.Vector3{X: cx + length/2, Y: cy + width/2, Z: model.Min.Z + height},
	}
}

// cylinderStock builds an explicit cylinder centred on the model's XY (represented by its bounding
// box: ±radius in XY, rising from the model bottom). Zero radius/height fall back to the model.
func cylinderStock(min, max []float64, radius, height float64) Stock {
	model := StockFromRangeBox(min, max)
	if radius <= 0 {
		radius = (model.Max.X - model.Min.X) / 2
	}
	if height <= 0 {
		height = model.Max.Z - model.Min.Z
	}
	cx, cy := (model.Min.X+model.Max.X)/2, (model.Min.Y+model.Max.Y)/2
	return Stock{
		Min: gcode.Vector3{X: cx - radius, Y: cy - radius, Z: model.Min.Z},
		Max: gcode.Vector3{X: cx + radius, Y: cy + radius, Z: model.Min.Z + height},
	}
}

// existingStock uses another body's range box as the stock; on a query failure it falls back to
// the machined body's range box.
func (e *Engine) existingStock(bodyIndex int, fallbackMin, fallbackMax []float64) Stock {
	rbox, err := e.api.Body().RangeBox(wire.BodyRangeBoxArgs{BodyIndex: bodyIndex, Precise: true})
	if err != nil {
		return StockFromRangeBox(fallbackMin, fallbackMax)
	}
	return StockFromRangeBox(rbox.Min, rbox.Max)
}
