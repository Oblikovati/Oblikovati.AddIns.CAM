// SPDX-License-Identifier: GPL-2.0-only

package feeds

import (
	"math"
	"testing"
)

// TestRecommendAluminium checks the RPM and feed for a 6mm 2-flute cutter in aluminium against
// the textbook formulae.
func TestRecommendAluminium(t *testing.T) {
	got, err := Recommend("aluminium", 6, 2)
	if err != nil {
		t.Fatalf("Recommend: %v", err)
	}
	// RPM = 200·1000 / (π·6) ≈ 10610.
	wantRPM := int(math.Round(200 * 1000 / (math.Pi * 6)))
	if got.RPM != wantRPM {
		t.Errorf("RPM = %d, want %d", got.RPM, wantRPM)
	}
	// feed = RPM · 2 flutes · 0.05 mm.
	wantFeed := math.Round(float64(wantRPM) * 2 * 0.05)
	if got.FeedRate != wantFeed {
		t.Errorf("feed = %g, want %g", got.FeedRate, wantFeed)
	}
}

// TestRecommendCapsRPM checks a tiny tool in a fast material is capped at the machine maximum,
// and the feed follows the capped RPM.
func TestRecommendCapsRPM(t *testing.T) {
	got, err := Recommend("plastic", 1, 2)
	if err != nil {
		t.Fatalf("Recommend: %v", err)
	}
	if got.RPM != machineMaxRPM {
		t.Errorf("RPM = %d, want it capped at %d", got.RPM, machineMaxRPM)
	}
	// The feed follows the capped RPM and the diameter-scaled chip load (1mm tool clamps to the
	// minimum chip-load scale).
	if want := math.Round(machineMaxRPM * 2 * 0.08 * chipLoadScale(1)); got.FeedRate != want {
		t.Errorf("feed = %g, want %g (from the capped RPM and scaled chip load)", got.FeedRate, want)
	}
}

// TestRecommendChipLoadScalesWithDiameter checks a larger tool takes a heavier chip per tooth than
// a small one in the same material — the feed-per-tooth (feed / (RPM·flutes)) rises with diameter.
func TestRecommendChipLoadScalesWithDiameter(t *testing.T) {
	small, _ := Recommend("aluminium", 3, 2)
	large, _ := Recommend("aluminium", 12, 2)
	fptSmall := small.FeedRate / (float64(small.RPM) * 2)
	fptLarge := large.FeedRate / (float64(large.RPM) * 2)
	if fptLarge <= fptSmall {
		t.Errorf("feed-per-tooth should grow with diameter: 3mm %.4f, 12mm %.4f", fptSmall, fptLarge)
	}
	// A reference-diameter tool is unscaled (scale exactly 1).
	if s := chipLoadScale(chipLoadRefDiameter); s != 1 {
		t.Errorf("a reference-diameter tool should scale by 1, got %g", s)
	}
}

// TestRecommendHarderMaterialIsSlower checks steel yields a lower RPM than aluminium.
func TestRecommendHarderMaterialIsSlower(t *testing.T) {
	al, _ := Recommend("aluminium", 6, 2)
	steel, _ := Recommend("steel", 6, 2)
	if steel.RPM >= al.RPM {
		t.Errorf("steel RPM %d should be below aluminium %d", steel.RPM, al.RPM)
	}
}

// TestRecommendErrors covers unknown material and degenerate tool inputs.
func TestRecommendErrors(t *testing.T) {
	if _, err := Recommend("unobtanium", 6, 2); err == nil {
		t.Error("an unknown material must error")
	}
	if _, err := Recommend("steel", 0, 2); err == nil {
		t.Error("a zero tool diameter must error")
	}
	if _, err := Recommend("steel", 6, 0); err == nil {
		t.Error("a zero flute count must error")
	}
}

// TestMaterialsSorted checks the catalogue lists its materials sorted and case-insensitively
// looks up.
func TestMaterialsSorted(t *testing.T) {
	mats := Materials()
	if len(mats) < 5 {
		t.Fatalf("catalogue has %d materials, want several", len(mats))
	}
	for i := 1; i < len(mats); i++ {
		if mats[i] < mats[i-1] {
			t.Errorf("materials not sorted: %v", mats)
		}
	}
	if _, ok := Lookup("Aluminium"); !ok {
		t.Error("Lookup should be case-insensitive")
	}
}
