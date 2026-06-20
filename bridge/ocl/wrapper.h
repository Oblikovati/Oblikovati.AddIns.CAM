/* SPDX-License-Identifier: GPL-2.0-only */
/* C ABI over OpenCAMLib's drop-cutter, so cgo can call it. The C++ side is OpenCAMLib
   (LGPL-2.1, see COPYING.opencamlib); this shim is the project's thin owned interface. */
#ifndef OBK_OCL_WRAPPER_H
#define OBK_OCL_WRAPPER_H

#ifdef __cplusplus
extern "C" {
#endif

/* obk_ocl_drop_lines drops a ball-nose cutter along nseg XY scan lines over a triangle mesh,
   following the surface down to minZ. tris holds 9 doubles per triangle (ax,ay,az, bx,by,bz,
   cx,cy,cz); segs holds 4 doubles per scan line (x0,y0, x1,y1). On success it returns the total
   cutter-location point count, writes the points as xyz triples to *out_xyz, and the per-line
   point count to *out_counts (nseg ints). Both out arrays are malloc'd and the caller frees them
   with obk_ocl_free_d / obk_ocl_free_i. Returns -1 on bad input or an internal error. */
int obk_ocl_drop_lines(const double *tris, int ntris,
                       double ball_diameter, double ball_length,
                       double min_z, double sampling,
                       const double *segs, int nseg,
                       double **out_xyz, int **out_counts);

void obk_ocl_free_d(double *p);
void obk_ocl_free_i(int *p);

#ifdef __cplusplus
}
#endif

#endif /* OBK_OCL_WRAPPER_H */
