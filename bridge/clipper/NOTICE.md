# Vendored Clipper (integer polygon-clipping engine)

This package embeds the **Clipper** library (`clipper.cpp`, `clipper.hpp`) to drive
the integer polygon **boolean** (union / difference / intersection) and **offset**
operations the adaptive high-speed-clearing solver is built on. Clipper is licensed
under the **Boost Software License 1.0** — see [`COPYING.clipper`](./COPYING.clipper).
Upstream: Angus Johnson, <http://www.angusj.com> (Clipper v6.4.2).

The `clipper.cpp` / `clipper.hpp` files keep their original Boost license headers
(`SPDX-License-Identifier: BSL-1.0`). Only the project-owned C-ABI shim (`wrapper.h`,
`wrapper.cpp`) and the Go files carry this project's `GPL-2.0-only` header. The Boost
license is permissive and imposes no copyleft, so static linking into this GPL add-in
is unencumbered.

## Why vendored, and why cgo

The library is a single translation unit (`clipper.cpp`); cgo compiles it directly,
so `go build` needs no CMake or prebuilt library and the build stays portable across
the CI matrix. The library is compiled with `use_xyz` (a `Z` member on `IntPoint`, the
upstream default in this copy) — the wrapper carries only X,Y and leaves Z at 0, since
2D clearing does not use it.

A non-cgo stub (`clipper_stub.go`) returns an error from the engine functions so the
rest of the add-in stays buildable/testable with `CGO_ENABLED=0`; the cheap standalone
predicates (`clipper.go`: Area, Orientation, PointInPolygon, ReversePath, CleanPolygon)
are pure Go and work in every build.

## Updating

Re-vendor by copying `clipper.cpp` and `clipper.hpp` from a clean Clipper v6.4.x
checkout (keeping `#define use_xyz` and the 64-bit `cInt`). The smoke test
(`clipper_cgo_test.go`) guards correctness across the boolean / offset / simplify
surface the wrapper exposes.
