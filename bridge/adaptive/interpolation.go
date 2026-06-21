// SPDX-License-Identifier: GPL-2.0-only

package adaptive

import (
	"math"

	"oblikovati.org/cam/bridge/clipper"
)

// interpMinAngle and interpMaxAngle bound the deflection angle the solver searches each step:
// ±45°. interpMinInterp keeps the linear interpolation from collapsing to a pure secant step, so
// the angle search stays between binary search (robust) and linear interpolation (fast).
const (
	interpMinAngle  = -math.Pi / 4
	interpMaxAngle  = math.Pi / 4
	interpMinInterp = 0.2
)

// interpItem is one sampled (deflection angle → resulting cut-area error) point: the angle and the
// tool position it implies, the signed error against the target cut area, and whether that move is
// conventional (vs climb) milling.
type interpItem struct {
	angle          float64
	point          clipper.IntPoint
	errorVal       float64
	isConventional bool
}

// interpolation tracks at most two bracketing samples of the cut-area error as a function of the
// deflection angle and interpolates the angle that hits the target. It is the solver's angle
// predictor — an exact port of the Adaptive2d Interpolation class — keeping the lower (min) and
// upper (max) error samples and preferring a climb-milling bracket.
type interpolation struct {
	min, max *interpItem
}

// clear forgets both samples (called at the start of each step's angle search).
func (ip *interpolation) clear() {
	ip.min, ip.max = nil, nil
}

// bothSides reports whether the two samples bracket zero error (min below, max at/above) with at
// least one of them climb-milling — the condition under which a true interpolation is meaningful.
func (ip *interpolation) bothSides() bool {
	return ip.min != nil && ip.max != nil && ip.min.errorVal < 0 && ip.max.errorVal >= 0 &&
		(!ip.min.isConventional || !ip.max.isConventional)
}

// getPointCount is how many samples are held (0, 1, or 2).
func (ip *interpolation) getPointCount() int {
	n := 0
	if ip.min != nil {
		n++
	}
	if ip.max != nil {
		n++
	}
	return n
}

// clampAngle restricts an angle to the searchable ±45° range.
func (ip *interpolation) clampAngle(angle float64) float64 {
	return math.Max(math.Min(angle, interpMaxAngle), interpMinAngle)
}

// addPoint records a sample, keeping the two that best bracket zero error while preferring a
// climb-milling bracket and, when allowSkip is set, discarding a sample that is worse than both
// held ones. Exact port — the conditional order is load-bearing.
func (ip *interpolation) addPoint(errorVal, angle float64, point clipper.IntPoint, allowSkip, isConventional bool) {
	newItem := interpItem{angle: angle, point: point, errorVal: errorVal, isConventional: isConventional}
	switch {
	case ip.min == nil:
		ip.min = &newItem
	case ip.max == nil:
		ip.max = &newItem
		if ip.min.errorVal > ip.max.errorVal {
			ip.min, ip.max = ip.max, ip.min
		}
	case isConventional && (ip.min.isConventional != ip.max.isConventional):
		if !allowSkip {
			ip.resetConventional()
			ip.addPoint(errorVal, angle, point, false, isConventional)
		}
	case ip.bothSides():
		if errorVal < 0 {
			ip.min = &newItem
		} else {
			ip.max = &newItem
		}
	default:
		if allowSkip && math.Abs(errorVal) > math.Abs(ip.min.errorVal) && math.Abs(errorVal) > math.Abs(ip.max.errorVal) &&
			(isConventional || !ip.min.isConventional || !ip.max.isConventional) {
			return
		}
		if ip.min.isConventional != ip.max.isConventional {
			ip.resetConventional()
		} else if math.Abs(ip.min.errorVal) > math.Abs(ip.max.errorVal) {
			ip.min = nil
		} else {
			ip.max = nil
		}
		ip.addPoint(errorVal, angle, point, false, isConventional)
	}
}

// resetConventional drops whichever held sample is the conventional one, so a climb sample can
// take its place.
func (ip *interpolation) resetConventional() {
	if ip.min.isConventional {
		ip.min = nil
	} else {
		ip.max = nil
	}
}

// interpolateAngle returns the deflection angle predicted to hit zero cut-area error: the
// max-engagement angle when only an upper sample is held, the min-engagement angle when only a
// lower one is, and the (fraction-clamped) linear interpolation between the two otherwise.
func (ip *interpolation) interpolateAngle() float64 {
	if ip.min == nil {
		return interpMinAngle
	}
	if ip.max == nil {
		return interpMaxAngle
	}
	p := (0 - ip.min.errorVal) / (ip.max.errorVal - ip.min.errorVal)
	p = math.Max(math.Min(p, 1-interpMinInterp), interpMinInterp)
	return ip.min.angle*(1-p) + ip.max.angle*p
}
