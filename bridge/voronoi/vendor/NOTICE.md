# Vendored third-party code: Boost.Polygon Voronoi

The headers under `boost/polygon/` are the **Boost.Polygon Voronoi library**
by Andrii Sydorchuk (Copyright 2010–2012), part of Boost. They are vendored
here verbatim and used only to construct the segment Voronoi diagram that the
V-carve toolpath rides — the same engine FreeCAD's CAM workbench uses for its
medial-axis V-carve, so our output matches that reference exactly.

- **License:** Boost Software License, Version 1.0 (BSL-1.0). See
  `LICENSE_1_0.txt` in this directory.
- **Files (unmodified upstream, from boost-1.84.0):**
  - `boost/polygon/voronoi_builder.hpp`
  - `boost/polygon/voronoi_diagram.hpp`
  - `boost/polygon/voronoi_geometry_type.hpp`
  - `boost/polygon/detail/voronoi_ctypes.hpp`
  - `boost/polygon/detail/voronoi_predicates.hpp`
  - `boost/polygon/detail/voronoi_structures.hpp`
  - `boost/polygon/detail/voronoi_robust_fpt.hpp`

`boost/cstdint.hpp` is **not** Boost code: it is a small project-owned shim
(GPL-2.0-only) that maps the four `boost::int*_t` typedefs the Voronoi headers
need onto C++11 `<cstdint>`, so the upstream headers compile with no other
Boost dependency.

The diagram is driven through `boost::polygon::voronoi_builder` directly
(`insert_point` / `insert_segment` with raw integer coordinates), which avoids
the `point_concept` / `segment_concept` headers entirely — hence the small
dependency closure above.
