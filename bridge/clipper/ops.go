// SPDX-License-Identifier: GPL-2.0-only

package clipper

// ClipType selects the boolean operation. The constant order matches ClipperLib's ClipType so
// the value can be passed straight across the C ABI.
type ClipType int

const (
	Intersection ClipType = iota
	Union
	Difference
	Xor
)

// FillType selects the polygon fill (winding) rule, matching ClipperLib's PolyFillType. EvenOdd
// is the usual choice for the disjoint regions the clearing solver works with.
type FillType int

const (
	EvenOdd FillType = iota
	NonZero
	Positive
	Negative
)

// JoinType selects how an offset turns a corner, matching ClipperLib's JoinType. Round is what
// the solver uses to model the tool's circular sweep.
type JoinType int

const (
	Square JoinType = iota
	Round
	Miter
)

// EndType selects how an offset treats a path's ends, matching ClipperLib's EndType. Closed
// polygons use ClosedPolygon; an open toolpath swept by a round tool uses OpenRound.
type EndType int

const (
	ClosedPolygon EndType = iota
	ClosedLine
	OpenButt
	OpenSquare
	OpenRound
)

// Unite returns the union of all the given closed-polygon sets (EvenOdd fill). It is the common
// case of merging the swept-tool coverage into the cleared-area model.
func Unite(subjects, clips Paths) (Paths, error) {
	return Boolean(Union, EvenOdd, subjects, true, clips, false)
}

// Subtract returns subjects minus clips (closed polygons, EvenOdd fill) — e.g. the stock region
// still to clear after removing what the tool has covered.
func Subtract(subjects, clips Paths) (Paths, error) {
	return Boolean(Difference, EvenOdd, subjects, true, clips, false)
}

// Intersect returns the overlap of subjects and clips (closed polygons, EvenOdd fill).
func Intersect(subjects, clips Paths) (Paths, error) {
	return Boolean(Intersection, EvenOdd, subjects, true, clips, false)
}

// OffsetClosed insets (delta<0) or outsets (delta>0) closed polygons with a round join — the
// solver's tool-radius offset of a boundary. Library default miter/arc knobs.
func OffsetClosed(paths Paths, delta float64) (Paths, error) {
	return Offset(paths, Round, ClosedPolygon, delta, 0, 0)
}
