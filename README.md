# Oblikovati CAM

A computer-aided-manufacturing (toolpath + G-code) add-in for
[Oblikovati](https://oblikovati.org). It defines a machining **Job** over a part —
stock, tools, and an ordered list of operations — generates the toolpaths, and posts
them to machine G-code for a range of controllers.

The add-in links **only** the public Apache-2.0 `oblikovati.org/api` and talks to the host
over the C ABI, so it loads as a self-contained shared library (`.so` / `.dll` / `.dylib`)
and never depends on the application internals.

```
manifest id:   com.oblikovati.cam
module:        oblikovati.org/cam   (GPL-2.0-only)
```

## Operations

Each operation reads its driving geometry from the part and emits a framed toolpath. They are
added and tuned from the **CAM** panel and the operation editor.

**2.5D clearing**
- **Pocket** — area clearing with two patterns (concentric **offset** rings or back-and-forth
  **zigzag** / one-direction), a wall **finish allowance**, and automatic routing around islands.
- **Adaptive** — high-speed (low-engagement) stay-down spiral clearing, island-aware, with a
  finish allowance.
- **Rest** — clears only the wall band a previous, larger tool could not reach (islands too).
- **Trochoidal** — overlapping circular loops marching along a contour, for deep slots in hard stock.
- **Slot** — a channel of a set width centred on a path.

**Contouring & engraving**
- **Profile** — inside / outside / on-the-line contouring with tool compensation, stock-to-leave,
  and multi-pass roughing; inner holes are profiled too.
- **Face** — facing the stock top with a raster or a continuous spiral.
- **Engrave** — follows a contour on the tool centre.
- **Chamfer / deburr** — single- or multi-pass bevel of an edge with a V-tool.
- **V-carve** — depth-varying relief carving with a V-bit.

**Holes**
- **Drilling** — G81/G82/G83/G85 canned cycles, peck/dwell/chip-break, blind depth, G98/G99
  retract mode, a nearest-neighbour travel tour, and a configurable spindle spin-up.
- **Tapping** — synchronised rigid tapping (G84 / G74).
- **Counterbore**, **Countersink**, **Thread mill**, **Helix bore**.

**3D finishing** (ball-nose, OpenCAMLib drop-cutter)
- **Surface** — parallel passes that ride the surface.
- **Crosshatch** — two perpendicular pass sets for a finer scallop.
- **Waterline** — constant-Z passes, good for steep walls.

**Probing** (G38.2 touch probing, G10 work/tool offsets)
- Workpiece corner, bore-centre, boss-centre, and tool-length probing.

**Custom** — raw operator-supplied G-code emitted verbatim, for macros the generators don't cover.

## Dress-ups

Toolpath post-processes applied after an operation is generated:
**holding tabs**, **dogbone** corner relief, **ramp** entry, **helical ramp** entry, and
**lead-in / lead-out** arcs.

## Post processors

`linuxcnc` · `grbl` · `fanuc` · `marlin` · `haas` · `heidenhain` (Klartext conversational).

The posted program opens with a tool-list setup sheet and per-operation cycle-time estimates,
shares tools across adjacent operations without redundant tool changes, and supports per-operation
optional stops (M1) and coolant (M7/M8/M9).

## Feeds & speeds

A built-in material catalogue (16 materials) recommends spindle speed and feed from the material,
tool diameter, and flute count, with diameter-based chip-load scaling.

## Visual gallery

Every toolpath feature has a rendered screenshot validating its output in
[`screenshots/`](screenshots/) — regenerate them from real operation output with:

```sh
go run ./cmd/camshot screenshots
```

## Layout

- `bridge/` — all behaviour, **cgo-free** so it unit-tests on every platform without a host:
  - `gcode` (toolpath model) · `geom2d` (2D polygon / offset / clip) · `gen` (pure toolpath
    generators) · `dressup` · `post` (G-code dialects) · `feeds` · `plot` (gallery renderer) ·
    `ocl` (drop-cutter, cgo) · the engine + operations.
- `cmd/camshot` — the gallery renderer. `cmd/camrun` — a headless job driver.
- `export.go` / `hostcaller.go` / `manifest.*` — the C ABI shell (the only cgo at the root).

## Build & test

```sh
make build      # build the c-shared add-in
make install    # build and install into the host's add-ins directory
make test       # run the test suite
go test ./...   # the pure-Go bridge tests (no host needed)
```
