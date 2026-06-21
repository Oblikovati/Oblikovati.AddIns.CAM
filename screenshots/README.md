# CAM feature gallery

Each image is the **actual generated toolpath** of a CAM operation — produced by running that
operation's `Execute` on a representative L-shaped part — rendered by `cmd/camshot`. Regenerate
with:

```
go run ./cmd/camshot screenshots
```

Legend: **red** = rapid (G0), **blue** = cutting move (G1/G2/G3), **orange** = cut above the floor
(tab lift / ramp), **green** = plunge / drilled point, **magenta** = G38 touch-probe move, **grey**
= the driving part boundary.

| Image | Validates |
|---|---|
| `profile.png` | Outside contour: the cut loop is the boundary grown by the tool radius. |
| `profile-multipass.png` | Profile roughing: several radial passes march from the stock inward to the final contour (for cutting a part out of thick stock). |
| `profile-holes.png` | Island-aware profiling: the outer outline cut outside plus each inner hole cut inside. |
| `pocket.png` | Area clearing: concentric inward rings, rapids between them. |
| `pocket-island.png` | Pocket with an island: the clearing routes around a central island, leaving it standing. |
| `pocket-finish.png` | Pocket with a finish allowance: roughing rings stop short of the wall (the allowance gap), then a final pass cleans the wall to size. |
| `adaptive.png` | HSM adaptive clearing: one continuous low-engagement spiral that stays down (no rapids between passes). |
| `adaptive-island.png` | Adaptive clearing with an island: the low-engagement passes route around a central island, leaving it standing. |
| `rest.png` | Rest machining: only the wall band a previous larger tool left (a few rings, interior untouched). |
| `rest-island.png` | Rest machining with an island: clears the wall band at the outer boundary AND the band hugging the island, leaving the already-cleared interior untouched. |
| `trochoidal.png` | Trochoidal milling: overlapping circular loops marching along the contour (low engagement). |
| `slot.png` | Slot / groove milling: a channel of a set width centred on the contour (passes straddle both sides). |
| `millface.png` | Facing: raster rows over the region. |
| `millface-spiral.png` | Facing (spiral pattern): one continuous inward spiral over the region, constant climb direction and a single plunge. |
| `engrave.png` | Engraving: the contour run on the tool centre (no compensation). |
| `chamfer.png` | Chamfer / edge-break: a single bevel pass offset by the chamfer width at the angle-derived depth. |
| `vcarve.png` | V-carve: nested contours deepening inward (the depth shade traces the V valley toward the spine). |
| `dressup-tabs.png` | Holding tabs lifting the tool over the boundary. |
| `dressup-dogbone.png` | Dogbone corner relief: 45° bones at the corners. |
| `dressup-ramp.png` | Ramp entry: the straight plunge replaced by an angled descent. |
| `dressup-helicalramp.png` | Helical ramp entry: the straight plunge replaced by a helix on a circle tangent to the first cut (the entry circle bored at the start). |
| `dressup-leadinout.png` | Lead in/out: plunge relocated off the contour, tangential arc entry/exit. |
| `drilling.png` | Drilling: canned-cycle points at each detected hole. |
| `probe.png` | Probing: G38.2 touch moves finding the stock top and two edges (magenta). |
| `boreprobe.png` | Bore-centre probing: four G38.2 touches from a hole centre out to the wall (a cross). |
| `bossprobe.png` | Boss-centre probing: four G38.2 touches inward from outside the outline to the walls. |
| `helix.png` | Helix bore: the tool-centre orbit for a hole wider than the tool. |
| `threadmill.png` | Thread milling: the thread orbit plus the lead-in/out arc easing on/off the thread. |
| `counterbore.png` | Counterbore / spot-face: concentric helical passes clearing a flat-bottom recess at the hole top. |
| `tapping.png` | Tapping: a synchronised tap cycle (G84) at each detected hole — feed = thread pitch × spindle rpm. |
| `countersink.png` | Countersink: a conical spiral from the rim down to the centre (the depth shade traces the cone). |
| `surface.png` | 3D surface finish: parallel zig-zag passes; the depth shade (orange high → blue low) shows the surface. |
| `waterline.png` | 3D waterline finish: nested constant-Z contour loops down the surface. |

The `surface` and `waterline` shots run on a synthetic pyramid surface so the 3D-finishing
toolpaths render without a mesh or the OpenCAMLib drop-cutter.
