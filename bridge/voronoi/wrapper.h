/* SPDX-License-Identifier: GPL-2.0-only */
/* C ABI over the Boost.Polygon Voronoi library (BSL-1.0, see vendor/NOTICE.md) so cgo can build a
   segment Voronoi diagram — the medial-axis engine the V-carve toolpath rides, the same engine the
   reference CAM workbench uses. Inputs cross flattened as scaled int64: `points` is 2 per point
   (x,y), `segments` is 4 per segment (x0,y0,x1,y1). Coordinates must fit in 32 bits after scaling.

   On success obk_voronoi_build returns the edge count (>=0) and writes two malloc'd arrays the
   caller frees: *out_coords holds 4 doubles per edge (v0x,v0y,v1x,v1y, in the scaled plane) and
   *out_meta holds 8 int64 per edge:
     [0] v0 valid (1) or infinite (0)
     [1] v1 valid (1) or infinite (0)
     [2] is_primary
     [3] is_linear
     [4] cell source index
     [5] cell source category (Boost SOURCE_CATEGORY_*: 0 point, 1/2 segment endpoint, 8/9 segment)
     [6] twin cell source index
     [7] twin cell source category
   Returns -1 on failure. */
#ifndef OBK_VORONOI_WRAPPER_H
#define OBK_VORONOI_WRAPPER_H

#ifdef __cplusplus
extern "C" {
#endif

int obk_voronoi_build(const long long *points, int npoints,
                      const long long *segments, int nsegments,
                      double **out_coords, long long **out_meta);

void obk_voronoi_free_d(double *p);
void obk_voronoi_free_ll(long long *p);

#ifdef __cplusplus
}
#endif

#endif /* OBK_VORONOI_WRAPPER_H */
