// SPDX-License-Identifier: GPL-2.0-only

package bridge

// cloned returns a copy of the base envelope with its own dressup slice and a label suffix.
// Geometry-bearing slices on the concrete operations (boundaries, holes, rows) are shared with
// the original — they are read-only after generation — but the dressup chain, which the editor
// mutates, is copied.
func (b OpBase) cloned(suffix string) OpBase {
	b.Dressups = append([]Dressup(nil), b.Dressups...)
	b.OpLabel += suffix
	return b
}

// Clone deep-copies an operation (label suffixed " copy") so the editor can duplicate it.

func (op *DrillingOp) Clone() Operation { cp := *op; cp.OpBase = op.cloned(" copy"); return &cp }
func (op *ProfileOp) Clone() Operation  { cp := *op; cp.OpBase = op.cloned(" copy"); return &cp }
func (op *PocketOp) Clone() Operation   { cp := *op; cp.OpBase = op.cloned(" copy"); return &cp }
func (op *AdaptiveOp) Clone() Operation { cp := *op; cp.OpBase = op.cloned(" copy"); return &cp }
func (op *RestOp) Clone() Operation     { cp := *op; cp.OpBase = op.cloned(" copy"); return &cp }
func (op *TrochoidalOp) Clone() Operation {
	cp := *op
	cp.OpBase = op.cloned(" copy")
	return &cp
}
func (op *MillFaceOp) Clone() Operation { cp := *op; cp.OpBase = op.cloned(" copy"); return &cp }
func (op *EngraveOp) Clone() Operation  { cp := *op; cp.OpBase = op.cloned(" copy"); return &cp }
func (op *ChamferOp) Clone() Operation  { cp := *op; cp.OpBase = op.cloned(" copy"); return &cp }
func (op *HelixOp) Clone() Operation    { cp := *op; cp.OpBase = op.cloned(" copy"); return &cp }
func (op *ThreadMillOp) Clone() Operation {
	cp := *op
	cp.OpBase = op.cloned(" copy")
	return &cp
}
func (op *SurfaceOp) Clone() Operation { cp := *op; cp.OpBase = op.cloned(" copy"); return &cp }
func (op *WaterlineOp) Clone() Operation {
	cp := *op
	cp.OpBase = op.cloned(" copy")
	return &cp
}
