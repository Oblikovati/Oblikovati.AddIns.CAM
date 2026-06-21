// SPDX-License-Identifier: GPL-2.0-only

package dressup

import "oblikovati.org/cam/bridge/gcode"

// cutRef is one cutting move within a loop: its command index in the path, the geometric
// move, and the feed in effect (so spliced bones inherit it).
type cutRef struct {
	index int
	mv    move
	feed  float64
}

// ApplyDogbone inserts corner-relief bones into a profile/pocket path so a round end mill can
// reach internal corners. It splits the path into cutting loops (broken by rapids), forms a
// kink at every corner, and for each corner that turns sharply enough on the selected side it
// cuts a bone out and back. A zero length returns the path unchanged.
func ApplyDogbone(path gcode.Path, p DogboneParams) gcode.Path {
	if p.Length <= 0 {
		return path
	}
	inserts := planDogbones(path, p)
	if len(inserts) == 0 {
		return path
	}
	return spliceAfter(clonePath(path), inserts)
}

// planDogbones returns the bone commands to insert keyed by the command index they follow.
func planDogbones(path gcode.Path, p DogboneParams) map[int][]gcode.Command {
	inserts := map[int][]gcode.Command{}
	for _, group := range cuttingLoops(path) {
		for _, k := range loopKinks(group) {
			if !qualifies(k.kink, p) {
				continue
			}
			in, out := generateBone(k.kink, p.Length, boneAngle(p.Style, k.kink))
			withFeed(&in, k.feed)
			withFeed(&out, k.feed)
			inserts[k.cornerIndex] = append(inserts[k.cornerIndex], in, out)
		}
	}
	return inserts
}

// cuttingLoops groups consecutive cutting moves (G1/G2/G3 carrying X or Y) into loops,
// breaking on any rapid (G0). Pure-Z plunges are skipped without breaking a loop.
func cuttingLoops(path gcode.Path) [][]cutRef {
	var loops [][]cutRef
	var cur []cutRef
	var px, py, pz float64
	posKnown := false
	flush := func() {
		if len(cur) > 0 {
			loops = append(loops, cur)
			cur = nil
		}
	}
	for i, c := range path.Commands {
		nx, ny, nz, hasXY := endpoint(c, px, py, pz)
		if c.Name == "G0" {
			flush()
		} else if hasXY && posKnown {
			cur = append(cur, cutRef{index: i, mv: moveOf(c, px, py, nx, ny), feed: c.Params["F"]})
		}
		px, py, pz = nx, ny, nz
		posKnown = posKnown || hasXY
	}
	flush()
	return loops
}

// cornerKink is a qualifying corner: the kink, the command index it follows, and the feed.
type cornerKink struct {
	kink        kink
	cornerIndex int
	feed        float64
}

// loopKinks builds the kink at each interior corner of a loop, plus the closing corner when
// the loop returns to its start.
func loopKinks(group []cutRef) []cornerKink {
	var kinks []cornerKink
	for j := 1; j < len(group); j++ {
		kinks = append(kinks, cornerKink{kink: newKink(group[j-1].mv, group[j].mv), cornerIndex: group[j-1].index, feed: group[j-1].feed})
	}
	if n := len(group); n >= 2 && roughlySame(group[0].mv.begin, group[n-1].mv.end) {
		kinks = append(kinks, cornerKink{kink: newKink(group[n-1].mv, group[0].mv), cornerIndex: group[n-1].index, feed: group[n-1].feed})
	}
	return kinks
}

// endpoint applies a command's X/Y/Z to the current position, reporting whether it moved in
// the XY plane (the test for a cutting move).
func endpoint(c gcode.Command, px, py, pz float64) (x, y, z float64, hasXY bool) {
	x, y, z = px, py, pz
	if v, ok := c.Params["X"]; ok {
		x, hasXY = v, true
	}
	if v, ok := c.Params["Y"]; ok {
		y, hasXY = v, true
	}
	if v, ok := c.Params["Z"]; ok {
		z = v
	}
	return x, y, z, hasXY
}

// moveOf builds the geometric move for a cutting command (G2 cw / G3 ccw become arcs).
func moveOf(c gcode.Command, px, py, nx, ny float64) move {
	begin := gcode.Vector3{X: px, Y: py}
	end := gcode.Vector3{X: nx, Y: ny}
	if c.Name == "G2" || c.Name == "G3" {
		return arcMove(begin, end, c.Params["I"], c.Params["J"], c.Name == "G3")
	}
	return straightMove(begin, end)
}

// withFeed adds the loop's feed to a bone move so the spliced G-code stays valid.
func withFeed(c *gcode.Command, feed float64) {
	if feed > 0 {
		c.Params["F"] = feed
	}
}

// spliceAfter rebuilds the path inserting each index's bone commands right after that command.
func spliceAfter(path gcode.Path, inserts map[int][]gcode.Command) gcode.Path {
	out := gcode.Path{Commands: make([]gcode.Command, 0, len(path.Commands))}
	for i, c := range path.Commands {
		out.Commands = append(out.Commands, c)
		out.Commands = append(out.Commands, inserts[i]...)
	}
	return out
}
