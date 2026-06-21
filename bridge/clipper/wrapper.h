/* SPDX-License-Identifier: GPL-2.0-only */
/* C ABI over the integer polygon-clipping engine, so cgo can call it. The C++ side is the
   Clipper library (BSL-1.0, see COPYING.clipper / NOTICE.md); this shim is the project's thin
   owned interface. Polygon sets cross the boundary flattened: `coords` holds 2 int64 per point
   (x,y interleaved), `counts` holds the point count of each of `npaths` paths. Every out array
   is malloc'd and the caller frees it with obk_clipper_free_i / obk_clipper_free_ll. */
#ifndef OBK_CLIPPER_WRAPPER_H
#define OBK_CLIPPER_WRAPPER_H

#ifdef __cplusplus
extern "C" {
#endif

/* obk_clipper_boolean clips a subject path set against a clip path set.
     clip_type : 0=intersection 1=union 2=difference 3=xor
     fill_type : 0=evenOdd 1=nonZero 2=positive 3=negative
     subj_closed : 1 to treat subjects as closed polygons, 0 as open polylines
     return_open : 0 => the closed-polygon solution; 1 => the OPEN paths the boolean produced
                   (executed via a PolyTree, used to clip an open path against a region)
   On success returns the number of result paths (>=0), writes their point counts to *out_counts
   (that many ints) and their x,y pairs to *out_coords (2*sum(counts) int64). Returns -1 on bad
   input or an internal failure. */
int obk_clipper_boolean(int clip_type, int fill_type,
                        const long long *subj, const int *subj_counts, int n_subj, int subj_closed,
                        const long long *clip, const int *clip_counts, int n_clip,
                        int return_open,
                        int **out_counts, long long **out_coords);

/* obk_clipper_offset insets/outsets a closed-polygon (or open-polyline) set by `delta` integer
   units (negative shrinks, positive grows).
     join_type : 0=square 1=round 2=miter
     end_type  : 0=closedPolygon 1=closedLine 2=openButt 3=openSquare 4=openRound
   miter_limit and arc_tolerance are the ClipperOffset knobs (pass <=0 for the library defaults
   of 2.0 / 0.25). Returns the result-path count and writes counts/coords as obk_clipper_boolean
   does; -1 on failure. */
int obk_clipper_offset(const long long *paths, const int *counts, int npaths,
                       int join_type, int end_type, double delta,
                       double miter_limit, double arc_tolerance,
                       int **out_counts, long long **out_coords);

/* obk_clipper_simplify resolves self-intersections in a path set (Clipper's SimplifyPolygons),
   fill_type as in obk_clipper_boolean. Returns the result-path count and writes counts/coords;
   -1 on failure. */
int obk_clipper_simplify(const long long *paths, const int *counts, int npaths, int fill_type,
                         int **out_counts, long long **out_coords);

void obk_clipper_free_i(int *p);
void obk_clipper_free_ll(long long *p);

#ifdef __cplusplus
}
#endif

#endif /* OBK_CLIPPER_WRAPPER_H */
