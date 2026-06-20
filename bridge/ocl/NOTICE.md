# Vendored OpenCAMLib (drop-cutter subset)

This package embeds a subset of **OpenCAMLib** to drive 3D surface finishing
(drop-cutter). OpenCAMLib is licensed **LGPL-2.1-or-later** — see
[`COPYING.opencamlib`](./COPYING.opencamlib). Upstream:
<https://github.com/aewallin/opencamlib>.

The `.hpp`/`.cpp` files in this directory keep their original OpenCAMLib license
headers. Only the project-owned C-ABI shim (`wrapper.h`, `wrapper.cpp`) and the Go
files carry this project's `GPL-2.0-only` header. Static linking of the LGPL library
into this GPL add-in is permitted; the whole add-in is open source, so the LGPL
relink requirement is satisfied.

## Why a subset, and why vendored

Only the **drop-cutter** path is used (`geo`, `cutters`, `dropcutter`, plus
`algo/{fiber,interval}` and `common/{numeric,lineclfilter}` — 26 translation units).
cgo compiles these directly, so `go build` needs no CMake or prebuilt library, and
the build stays portable across the CI matrix. A non-cgo stub (`dropcutter_stub.go`)
keeps the rest of the add-in buildable/testable with `CGO_ENABLED=0`.

## Boost removed

Upstream links Boost. The drop-cutter subset only reaches Boost trivially, and all of
it has been eliminated so there is **no Boost dependency**:

- `boost/foreach.hpp` → [`shim/boost/foreach.hpp`](./shim/boost/foreach.hpp): a one-line
  `BOOST_FOREACH` → C++11 range-for.
- `boost/math/special_functions/fpclassify.hpp` →
  [`shim/boost/math/special_functions/fpclassify.hpp`](./shim/boost/math/special_functions/fpclassify.hpp):
  `boost::math::isnan/isinf` → `std::`.
- `boost/graph` is reached only by the **waterline/weave** code (not vendored). The one
  leak was `interval.hpp` declaring a weave-only `WeaveVertex` via
  `boost::adjacency_list_traits<…>::vertex_descriptor`; that typedef is patched to
  `typedef void* WeaveVertex;`, its `halfedgediagram.hpp` include dropped, and an explicit
  `#include <set>` added. Search `[oblikovati]` in `interval.hpp` for the patch.

`<cassert>` is force-included via `CXXFLAGS: -include cassert` (it was previously pulled
in transitively through the Boost headers).

## Updating

Re-vendor from a clean OpenCAMLib checkout by copying the 26 `.cpp` (and the `.hpp` from
`geo`, `cutters`, `dropcutter`, `common`, plus `algo/{operation,fiber,interval}.hpp`),
then re-applying the `interval.hpp` patch above. The smoke test
(`dropcutter_cgo_test.go`) guards correctness.
