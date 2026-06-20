// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "oblikovati.org/cam/bridge/gcode"

// cmToMM converts the host's centimetre database unit to the millimetres CAM and G-code
// work in. Lengths only leave the kernel's cm convention here, at the CAM boundary (the
// same single-conversion-point discipline the host's exchange layer uses).
const cmToMM = 10.0

// Stock is the raw material the Job machines from: an axis-aligned box in millimetres.
// In milestone 1 it is derived directly from the part's range box (a tight billet);
// box/cylinder/from-base stock with offsets is a later refinement. Mirrors FreeCAD's
// Path/Main/Stock.
type Stock struct {
	Min gcode.Vector3 // mm
	Max gcode.Vector3 // mm
}

// StockFromRangeBox builds stock from a host BodyRangeBox result (min/max in cm),
// converting to millimetres. min and max are [x,y,z] triplets as returned by
// client.Body().RangeBox; a malformed (short) slice yields the zero box.
func StockFromRangeBox(min, max []float64) Stock {
	if len(min) < 3 || len(max) < 3 {
		return Stock{}
	}
	return Stock{
		Min: gcode.Vector3{X: min[0] * cmToMM, Y: min[1] * cmToMM, Z: min[2] * cmToMM},
		Max: gcode.Vector3{X: max[0] * cmToMM, Y: max[1] * cmToMM, Z: max[2] * cmToMM},
	}
}

// TopZ is the stock's upper Z (mm) — the reference plane operations rapid above and
// measure depths down from.
func (s Stock) TopZ() float64 { return s.Max.Z }

// BottomZ is the stock's lower Z (mm).
func (s Stock) BottomZ() float64 { return s.Min.Z }
