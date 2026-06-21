// SPDX-License-Identifier: GPL-2.0-only

// C-ABI wrapper over the integer polygon-clipping engine (the C++ library is Clipper, BSL-1.0,
// see COPYING.clipper / NOTICE.md). It marshals the flat (counts, coords) representation the Go
// side uses into ClipperLib's Paths and back, and runs the boolean / offset / simplify the
// adaptive-clearing solver needs. The library is built with use_xyz (a Z member on IntPoint);
// the wrapper carries only X,Y and leaves Z at its 0 default, since 2D clearing does not use it.

#include "wrapper.h"

#include "clipper.hpp"

#include <cstdlib>
#include <cstring>
#include <vector>

using namespace ClipperLib;

namespace
{
// toPaths rebuilds the flat (coords,counts) pair into ClipperLib Paths. coords holds 2 int64 per
// point; counts[i] is the point count of path i.
Paths toPaths(const long long *coords, const int *counts, int npaths)
{
    Paths out(npaths);
    const long long *c = coords;
    for (int i = 0; i < npaths; ++i) {
        out[i].reserve(counts[i]);
        for (int j = 0; j < counts[i]; ++j) {
            out[i].push_back(IntPoint(static_cast<cInt>(c[0]), static_cast<cInt>(c[1])));
            c += 2;
        }
    }
    return out;
}

// emit flattens a Paths result into freshly malloc'd counts + coords arrays the caller frees.
// Returns the path count, or -1 if allocation fails.
int emit(const Paths &paths, int **out_counts, long long **out_coords)
{
    int npaths = static_cast<int>(paths.size());
    size_t totalPts = 0;
    for (const Path &p : paths) {
        totalPts += p.size();
    }
    // Always allocate at least one element so the malloc never returns NULL for an empty result.
    int *oc = static_cast<int *>(std::malloc((npaths ? npaths : 1) * sizeof(int)));
    long long *ox = static_cast<long long *>(std::malloc((totalPts ? totalPts * 2 : 1) * sizeof(long long)));
    if (!oc || !ox) {
        std::free(oc);
        std::free(ox);
        return -1;
    }
    long long *x = ox;
    for (int i = 0; i < npaths; ++i) {
        oc[i] = static_cast<int>(paths[i].size());
        for (const IntPoint &pt : paths[i]) {
            *x++ = static_cast<long long>(pt.X);
            *x++ = static_cast<long long>(pt.Y);
        }
    }
    *out_counts = oc;
    *out_coords = ox;
    return npaths;
}
}  // namespace

extern "C" int obk_clipper_boolean(int clip_type, int fill_type,
                                   const long long *subj, const int *subj_counts, int n_subj, int subj_closed,
                                   const long long *clip, const int *clip_counts, int n_clip,
                                   int return_open,
                                   int **out_counts, long long **out_coords)
{
    if (!out_counts || !out_coords || clip_type < 0 || clip_type > 3 || fill_type < 0 || fill_type > 3) {
        return -1;
    }
    if (n_subj <= 0 && n_clip <= 0) {
        return emit(Paths(), out_counts, out_coords);  // nothing to clip -> empty, not an error
    }
    try {
        Clipper clpr;
        if (n_subj > 0) {
            clpr.AddPaths(toPaths(subj, subj_counts, n_subj), ptSubject, subj_closed != 0);
        }
        if (n_clip > 0) {
            clpr.AddPaths(toPaths(clip, clip_counts, n_clip), ptClip, true);
        }
        Paths result;
        if (return_open) {
            PolyTree tree;
            if (!clpr.Execute(static_cast<ClipType>(clip_type), tree, static_cast<PolyFillType>(fill_type))) {
                return -1;
            }
            OpenPathsFromPolyTree(tree, result);
        } else if (!clpr.Execute(static_cast<ClipType>(clip_type), result, static_cast<PolyFillType>(fill_type))) {
            return -1;
        }
        return emit(result, out_counts, out_coords);
    } catch (...) {
        return -1;
    }
}

extern "C" int obk_clipper_offset(const long long *paths, const int *counts, int npaths,
                                  int join_type, int end_type, double delta,
                                  double miter_limit, double arc_tolerance,
                                  int **out_counts, long long **out_coords)
{
    if (!out_counts || !out_coords || npaths < 0 || join_type < 0 || join_type > 2 || end_type < 0 || end_type > 4) {
        return -1;
    }
    try {
        ClipperOffset co;
        if (miter_limit > 0) {
            co.MiterLimit = miter_limit;
        }
        if (arc_tolerance > 0) {
            co.ArcTolerance = arc_tolerance;
        }
        if (npaths > 0) {
            co.AddPaths(toPaths(paths, counts, npaths), static_cast<JoinType>(join_type), static_cast<EndType>(end_type));
        }
        Paths result;
        co.Execute(result, delta);
        return emit(result, out_counts, out_coords);
    } catch (...) {
        return -1;
    }
}

extern "C" int obk_clipper_simplify(const long long *paths, const int *counts, int npaths, int fill_type,
                                    int **out_counts, long long **out_coords)
{
    if (!out_counts || !out_coords || npaths < 0 || fill_type < 0 || fill_type > 3) {
        return -1;
    }
    try {
        Paths in = npaths > 0 ? toPaths(paths, counts, npaths) : Paths();
        Paths result;
        SimplifyPolygons(in, result, static_cast<PolyFillType>(fill_type));
        return emit(result, out_counts, out_coords);
    } catch (...) {
        return -1;
    }
}

extern "C" void obk_clipper_free_i(int *p) { std::free(p); }
extern "C" void obk_clipper_free_ll(long long *p) { std::free(p); }
