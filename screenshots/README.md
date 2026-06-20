# CAM feature gallery

Each image is the **actual generated toolpath** of a CAM operation — produced by running that
operation's `Execute` on a representative L-shaped part — rendered by `cmd/camshot`. Regenerate
with:

```
go run ./cmd/camshot screenshots
```

Legend: **red** = rapid (G0), **blue** = cutting move (G1/G2/G3), **green** = plunge / drilled
point, **grey** = the driving part boundary.

| Image | Validates |
|---|---|
| `profile.png` | Outside contour: the cut loop is the boundary grown by the tool radius. |
| `pocket.png` | Area clearing: concentric inward rings, rapids between them. |
| `adaptive.png` | HSM adaptive clearing: one continuous low-engagement spiral that stays down (no rapids between passes). |
| `rest.png` | Rest machining: only the wall band a previous larger tool left (a few rings, interior untouched). |
| `millface.png` | Facing: raster rows over the region. |
| `engrave.png` | Engraving: the contour run on the tool centre (no compensation). |
| `dressup-tabs.png` | Holding tabs lifting the tool over the boundary. |
| `dressup-dogbone.png` | Dogbone corner relief: 45° bones at the corners. |
| `dressup-ramp.png` | Ramp entry: the straight plunge replaced by an angled descent. |
| `dressup-leadinout.png` | Lead in/out: plunge relocated off the contour, tangential arc entry/exit. |
| `drilling.png` | Drilling: canned-cycle points at each detected hole. |
