// SPDX-License-Identifier: GPL-2.0-only

package gen

import "math"

// chainKey quantises a point to ~micron resolution so coincident segment endpoints (produced
// independently by neighbouring cells) match exactly when chained.
type chainKey struct{ x, y int64 }

// quantise maps a point to its chain key.
func quantise(p [2]float64) chainKey {
	return chainKey{int64(math.Round(p[0] * 1000)), int64(math.Round(p[1] * 1000))}
}

// chainSegments joins iso-line segments end-to-end into polylines/loops by matching shared
// endpoints — turning the unordered marching-squares output into walkable contours.
func chainSegments(segs [][2][2]float64) [][][2]float64 {
	adj := map[chainKey][]int{}
	for i, s := range segs {
		adj[quantise(s[0])] = append(adj[quantise(s[0])], i)
		adj[quantise(s[1])] = append(adj[quantise(s[1])], i)
	}
	used := make([]bool, len(segs))
	var loops [][][2]float64
	for start := range segs {
		if used[start] {
			continue
		}
		used[start] = true
		poly := [][2]float64{segs[start][0], segs[start][1]}
		cur := segs[start][1]
		for {
			next := nextSegment(adj, used, cur)
			if next < 0 {
				break
			}
			used[next] = true
			cur = farEndpoint(segs[next], cur)
			poly = append(poly, cur)
			if quantise(cur) == quantise(poly[0]) {
				break // closed the loop
			}
		}
		loops = append(loops, poly)
	}
	return loops
}

// nextSegment returns an unused segment index touching point cur, or -1.
func nextSegment(adj map[chainKey][]int, used []bool, cur [2]float64) int {
	for _, idx := range adj[quantise(cur)] {
		if !used[idx] {
			return idx
		}
	}
	return -1
}

// farEndpoint returns the segment endpoint that is not cur (the one to continue to).
func farEndpoint(seg [2][2]float64, cur [2]float64) [2]float64 {
	if quantise(seg[0]) == quantise(cur) {
		return seg[1]
	}
	return seg[0]
}
