// SPDX-License-Identifier: GPL-2.0-only

package gcode

// Canned-cycle expansion: a CNC controller interprets a modal drilling/tapping cycle (G81…G89,
// G73/G74) as the concrete rapid+feed motion of plunging each hole, but the raw program only carries
// the opaque cycle word. Consumers that reason about motion — the toolpath simulator, a backplot, a
// time estimate — need that motion made explicit, otherwise drilled/tapped holes are invisible to
// them. ExpandCannedCycles performs that interpretation, leaving every other command untouched.
//
// Assumes the absolute (G90), XY-plane (G17) programs that CAM posts emit. Only the full-depth
// plunge is reconstructed (not the peck sub-moves of G83/G73): the swept tool removes the whole hole
// from the plunge alone, and the peck detail does not change the removed volume.

// cannedCycles is the set of modal canned-cycle G-words expanded to plunge motion.
var cannedCycles = map[string]bool{
	"G73": true, "G74": true, "G76": true, "G81": true, "G82": true, "G83": true,
	"G84": true, "G85": true, "G86": true, "G87": true, "G88": true, "G89": true,
}

// ExpandCannedCycles rewrites canned drilling/tapping cycles into explicit G0/G1 plunge motion,
// resolving the modal state (active cycle, R plane, hole depth, G98/G99 retract). Non-cycle commands
// pass through unchanged.
func ExpandCannedCycles(path Path) Path {
	st := &cycleState{}
	out := make([]Command, 0, len(path.Commands))
	for _, c := range path.Commands {
		out = st.step(c, out)
	}
	return Path{Commands: out}
}

// peckClearance is how far (mm) a peck cycle backs off the previous cut depth — the small chip-break
// retract of G73, and the rapid-reapproach gap of G83 — kept below the peck increment.
const peckClearance = 0.5

// cycleState is the modal interpreter state carried across the program.
type cycleState struct {
	active           string  // the active canned cycle, "" when none
	x, y, z          float64 // sticky tool position
	r, bottom, q     float64 // modal retract plane, hole bottom, and peck increment (Q)
	dwell            float64 // modal bottom dwell (P), seconds — a tap holds here for spindle reversal
	initialZ         float64 // Z before the cycle block — the G98 retract target
	retractToInitial bool    // G98 (true) retracts to initialZ; G99 (false) to the R plane
}

// step folds one command into the output: expand a cycle, repeat it for a bare point, track the
// retract mode and cancellation, or pass the command through.
func (st *cycleState) step(c Command, out []Command) []Command {
	switch {
	case cannedCycles[c.Name]:
		return st.beginCycle(c, out)
	case st.active != "" && c.Name == "" && hasXY(c):
		return st.emitCycle(c, out)
	case c.Name == "G80":
		st.active = ""
		return append(out, c)
	case c.Name == "G98" || c.Name == "G99":
		st.retractToInitial = c.Name == "G98"
		return append(out, c)
	default:
		st.track(c)
		st.active = ""
		return append(out, c)
	}
}

// beginCycle starts (or re-words) a canned cycle, snapshotting the initial plane on the first cycle
// of a block and latching its modal R plane and hole depth.
func (st *cycleState) beginCycle(c Command, out []Command) []Command {
	if st.active == "" {
		st.initialZ = st.z
	}
	st.active = c.Name
	if r, ok := c.Params["R"]; ok {
		st.r = r
	}
	if z, ok := c.Params["Z"]; ok {
		st.bottom = z
	}
	if q, ok := c.Params["Q"]; ok {
		st.q = q
	}
	if p, ok := c.Params["P"]; ok {
		st.dwell = p
	}
	return st.emitCycle(c, out)
}

// emitCycle appends one hole's motion at the command's X/Y (modal otherwise): position over the hole,
// then the descent appropriate to the cycle — a single plunge, the peck woodpecker, or a tap that
// threads back out at feed.
func (st *cycleState) emitCycle(c Command, out []Command) []Command {
	st.setXY(c)
	retract := st.retractTarget()
	out = append(out, NewCommand("G0", map[string]float64{"X": st.x, "Y": st.y}))
	switch {
	case st.isPeck():
		out = append(st.emitPecks(out), rapid("Z", retract))
	case st.isTap():
		out = st.emitTap(out, retract)
	default:
		out = append(out, rapid("Z", st.r), feed("Z", st.bottom), rapid("Z", retract))
	}
	st.z = retract
	return out
}

// emitTap threads the tap in at feed, optionally dwells at the bottom for the spindle to reverse,
// then threads back out at feed (not a rapid — the tap is engaged in the thread the whole way).
func (st *cycleState) emitTap(out []Command, retract float64) []Command {
	out = append(out, rapid("Z", st.r), feed("Z", st.bottom))
	if st.dwell > 0 {
		out = append(out, feed("Z", st.bottom)) // hold at the bottom while the spindle reverses
	}
	return append(out, feed("Z", retract))
}

// setXY latches the cycle point's X/Y, keeping the modal value where the command omits one.
func (st *cycleState) setXY(c Command) {
	if x, ok := c.Params["X"]; ok {
		st.x = x
	}
	if y, ok := c.Params["Y"]; ok {
		st.y = y
	}
}

// retractTarget is where the cycle retracts to: the initial plane under G98, else the R plane.
func (st *cycleState) retractTarget() float64 {
	if st.retractToInitial {
		return st.initialZ
	}
	return st.r
}

// isPeck reports whether the active cycle drills in increments (G83 deep-hole, G73 chip-break) with
// a positive peck size.
func (st *cycleState) isPeck() bool {
	return st.q > 0 && (st.active == "G83" || st.active == "G73")
}

// isTap reports whether the active cycle is a tapping cycle (right-hand G84 or left-hand G74).
func (st *cycleState) isTap() bool {
	return st.active == "G84" || st.active == "G74"
}

// emitPecks appends the incremental descent to the hole bottom. G83 fully retracts to the R plane
// between pecks to clear chips and rapids back down to just above the last cut; G73 only backs off a
// little, staying in the hole.
func (st *cycleState) emitPecks(out []Command) []Command {
	out = append(out, rapid("Z", st.r))
	depths := peckDepths(st.r, st.bottom, st.q)
	for i, d := range depths {
		if i > 0 && st.active == "G83" {
			out = append(out, rapid("Z", depths[i-1]+peckBackoff(st.q)))
		}
		out = append(out, feed("Z", d))
		if i < len(depths)-1 {
			out = append(out, st.interPeck(d))
		}
	}
	return out
}

// interPeck is the move between two pecks: a full rapid retract to the R plane for G83, or a small
// chip-break back-off that stays in the hole for G73.
func (st *cycleState) interPeck(depth float64) Command {
	if st.active == "G83" {
		return rapid("Z", st.r)
	}
	return rapid("Z", depth+peckBackoff(st.q))
}

// peckDepths lists the feed-to depths from the R plane down to the bottom in q increments, the last
// landing exactly on the bottom.
func peckDepths(r, bottom, q float64) []float64 {
	var depths []float64
	cur := r
	for cur-q > bottom {
		cur -= q
		depths = append(depths, cur)
	}
	return append(depths, bottom)
}

// peckBackoff is the back-off distance for a peck, capped below the increment so it never overshoots.
func peckBackoff(q float64) float64 {
	if q/2 < peckClearance {
		return q / 2
	}
	return peckClearance
}

// rapid and feed build a single-axis rapid (G0) or feed (G1) move.
func rapid(axis string, v float64) Command { return NewCommand("G0", map[string]float64{axis: v}) }
func feed(axis string, v float64) Command  { return NewCommand("G1", map[string]float64{axis: v}) }

// track updates the sticky tool position from a pass-through command.
func (st *cycleState) track(c Command) {
	if x, ok := c.Params["X"]; ok {
		st.x = x
	}
	if y, ok := c.Params["Y"]; ok {
		st.y = y
	}
	if z, ok := c.Params["Z"]; ok {
		st.z = z
	}
}

// hasXY reports whether a command carries an X or Y address (a bare drilling point).
func hasXY(c Command) bool {
	_, x := c.Params["X"]
	_, y := c.Params["Y"]
	return x || y
}
