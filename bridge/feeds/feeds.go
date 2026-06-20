// SPDX-License-Identifier: GPL-2.0-only

// Package feeds computes recommended spindle speed and cutting feed from the workpiece material
// and the tool, so an operation can run at sane feeds and speeds instead of fixed defaults. It is
// a pure calculator with a small built-in material catalogue — no host or kernel dependency.
package feeds

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Material is a workpiece material's recommended cutting data for an end mill: SurfaceSpeed is
// the cutting (surface) speed Vc in metres/minute, and ChipLoad the feed per tooth fz in
// millimetres. These drive the RPM and feed calculation.
type Material struct {
	Name         string
	SurfaceSpeed float64 // Vc, m/min
	ChipLoad     float64 // fz, mm/tooth
}

// machineMaxRPM caps the recommended spindle speed to a typical router/mill ceiling; a small
// tool in a fast material would otherwise demand an impossible RPM.
const machineMaxRPM = 24000

// catalogue is the built-in material table (typical carbide end-mill values).
var catalogue = map[string]Material{
	"aluminium": {Name: "aluminium", SurfaceSpeed: 200, ChipLoad: 0.05},
	"brass":     {Name: "brass", SurfaceSpeed: 120, ChipLoad: 0.05},
	"steel":     {Name: "steel", SurfaceSpeed: 30, ChipLoad: 0.03},
	"stainless": {Name: "stainless", SurfaceSpeed: 20, ChipLoad: 0.02},
	"plastic":   {Name: "plastic", SurfaceSpeed: 300, ChipLoad: 0.08},
	"hardwood":  {Name: "hardwood", SurfaceSpeed: 250, ChipLoad: 0.10},
	"softwood":  {Name: "softwood", SurfaceSpeed: 300, ChipLoad: 0.12},
}

// Result is a recommended spindle speed (rev/min) and cutting feed (mm/min).
type Result struct {
	RPM      int
	FeedRate float64
}

// Materials returns the catalogue's material names, sorted, for a UI list.
func Materials() []string {
	names := make([]string, 0, len(catalogue))
	for n := range catalogue {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Lookup returns a material by (case-insensitive) name.
func Lookup(name string) (Material, bool) {
	m, ok := catalogue[strings.ToLower(strings.TrimSpace(name))]
	return m, ok
}

// Recommend computes the spindle speed and cutting feed for a tool of the given diameter (mm)
// and flute count cutting the named material. RPM = Vc·1000 / (π·D), capped at the machine
// maximum, and feed = RPM · flutes · chipload. Errors on an unknown material or non-positive
// tool diameter / flute count.
func Recommend(material string, toolDiameterMM float64, flutes int) (Result, error) {
	m, ok := Lookup(material)
	if !ok {
		return Result{}, fmt.Errorf("unknown material %q (have: %s)", material, strings.Join(Materials(), ", "))
	}
	if toolDiameterMM <= 0 {
		return Result{}, fmt.Errorf("tool diameter must be positive, got %g", toolDiameterMM)
	}
	if flutes <= 0 {
		return Result{}, fmt.Errorf("flute count must be positive, got %d", flutes)
	}
	rpm := math.Min(m.SurfaceSpeed*1000/(math.Pi*toolDiameterMM), machineMaxRPM)
	feed := rpm * float64(flutes) * m.ChipLoad
	return Result{RPM: int(math.Round(rpm)), FeedRate: math.Round(feed)}, nil
}
