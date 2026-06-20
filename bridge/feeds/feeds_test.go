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
	if want := math.Round(machineMaxRPM * 2 * 0.08); got.FeedRate != want {
		t.Errorf("feed = %g, want %g (from the capped RPM)", got.FeedRate, want)
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
