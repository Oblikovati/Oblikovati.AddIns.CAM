// SPDX-License-Identifier: GPL-2.0-only

//go:build !cgo

package clipper

import "testing"

// In a non-cgo build the engine functions must fail cleanly (not panic), so callers can fall
// back. The pure-Go predicates are exercised by clipper_test.go in every build.
func TestEngineStubsReturnError(t *testing.T) {
	sq := Paths{{{0, 0}, {10, 0}, {10, 10}, {0, 10}}}
	if _, err := Boolean(Union, EvenOdd, sq, true, sq, false); err == nil {
		t.Fatal("Boolean stub should return an error under CGO_ENABLED=0")
	}
	if _, err := Offset(sq, Round, ClosedPolygon, -1, 0, 0); err == nil {
		t.Fatal("Offset stub should return an error under CGO_ENABLED=0")
	}
	if _, err := Simplify(sq, EvenOdd); err == nil {
		t.Fatal("Simplify stub should return an error under CGO_ENABLED=0")
	}
	if _, err := PathIntersectArea(sq[0], sq); err == nil {
		t.Fatal("PathIntersectArea stub should return an error under CGO_ENABLED=0")
	}
}
