// SPDX-License-Identifier: GPL-2.0-only

// C-ABI wrapper over the Boost.Polygon Voronoi library (vendored, BSL-1.0, see vendor/NOTICE.md).
// It drives boost::polygon::voronoi_builder directly with raw integer coordinates — which avoids the
// point_concept / segment_concept headers — and flattens the resulting diagram's edge table into the
// (coords, meta) arrays the Go side reads. The medial-axis extraction and the per-vertex clearance
// distances are computed in Go from this table plus the original inputs, keeping this shim thin.

#include "wrapper.h"

#include "boost/polygon/voronoi_builder.hpp"
#include "boost/polygon/voronoi_diagram.hpp"

#include <cstdint>
#include <cstdlib>

namespace
{
using builder_type = boost::polygon::voronoi_builder<std::int32_t>;
using diagram_type = boost::polygon::voronoi_diagram<double>;
}  // namespace

extern "C" int obk_voronoi_build(const long long *points, int npoints,
                                 const long long *segments, int nsegments,
                                 double **out_coords, long long **out_meta)
{
    if (!out_coords || !out_meta || npoints < 0 || nsegments < 0) {
        return -1;
    }
    try {
        builder_type vb;
        for (int i = 0; i < npoints; ++i) {
            vb.insert_point(static_cast<std::int32_t>(points[2 * i]),
                            static_cast<std::int32_t>(points[2 * i + 1]));
        }
        for (int i = 0; i < nsegments; ++i) {
            vb.insert_segment(static_cast<std::int32_t>(segments[4 * i]),
                              static_cast<std::int32_t>(segments[4 * i + 1]),
                              static_cast<std::int32_t>(segments[4 * i + 2]),
                              static_cast<std::int32_t>(segments[4 * i + 3]));
        }
        diagram_type vd;
        vb.construct(&vd);

        int n = static_cast<int>(vd.num_edges());
        // Always allocate at least one element so malloc never returns NULL for an empty diagram.
        double *coords = static_cast<double *>(std::malloc((n ? 4 * n : 1) * sizeof(double)));
        long long *meta = static_cast<long long *>(std::malloc((n ? 8 * n : 1) * sizeof(long long)));
        if (!coords || !meta) {
            std::free(coords);
            std::free(meta);
            return -1;
        }

        int k = 0;
        for (diagram_type::const_edge_iterator it = vd.edges().begin(); it != vd.edges().end(); ++it) {
            const diagram_type::edge_type &e = *it;
            const diagram_type::vertex_type *v0 = e.vertex0();
            const diagram_type::vertex_type *v1 = e.vertex1();
            coords[4 * k + 0] = v0 ? v0->x() : 0.0;
            coords[4 * k + 1] = v0 ? v0->y() : 0.0;
            coords[4 * k + 2] = v1 ? v1->x() : 0.0;
            coords[4 * k + 3] = v1 ? v1->y() : 0.0;
            meta[8 * k + 0] = v0 ? 1 : 0;
            meta[8 * k + 1] = v1 ? 1 : 0;
            meta[8 * k + 2] = e.is_primary() ? 1 : 0;
            meta[8 * k + 3] = e.is_linear() ? 1 : 0;
            meta[8 * k + 4] = static_cast<long long>(e.cell()->source_index());
            meta[8 * k + 5] = static_cast<long long>(e.cell()->source_category());
            meta[8 * k + 6] = static_cast<long long>(e.twin()->cell()->source_index());
            meta[8 * k + 7] = static_cast<long long>(e.twin()->cell()->source_category());
            ++k;
        }
        *out_coords = coords;
        *out_meta = meta;
        return n;
    } catch (...) {
        return -1;
    }
}

extern "C" void obk_voronoi_free_d(double *p) { std::free(p); }
extern "C" void obk_voronoi_free_ll(long long *p) { std::free(p); }
