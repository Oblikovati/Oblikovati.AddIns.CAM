// SPDX-License-Identifier: GPL-2.0-only

// C-ABI wrapper over OpenCAMLib's PathDropCutter (the C++ library is LGPL-2.1, see
// COPYING.opencamlib). It builds the surface kd-tree once and drops the cutter along each scan
// line, the way a CAM surface op drives OCL.

#include "wrapper.h"

#include "ballcutter.hpp"
#include "clpoint.hpp"
#include "line.hpp"
#include "path.hpp"
#include "pathdropcutter.hpp"
#include "point.hpp"
#include "stlsurf.hpp"
#include "triangle.hpp"

#include <cstdlib>
#include <cstring>
#include <vector>

using namespace ocl;

extern "C" int obk_ocl_drop_lines(const double *tris, int ntris,
                                  double ball_diameter, double ball_length,
                                  double min_z, double sampling,
                                  const double *segs, int nseg,
                                  double **out_xyz, int **out_counts) {
    if (!tris || ntris <= 0 || !segs || nseg <= 0 || ball_diameter <= 0 || sampling <= 0 || !out_xyz || !out_counts) {
        return -1;
    }
    try {
        STLSurf surf;
        for (int i = 0; i < ntris; ++i) {
            const double *t = tris + 9 * i;
            surf.addTriangle(Triangle(Point(t[0], t[1], t[2]), Point(t[3], t[4], t[5]), Point(t[6], t[7], t[8])));
        }
        BallCutter cutter(ball_diameter, ball_length);
        PathDropCutter pdc;
        pdc.setSTL(surf); // builds the kd-tree once
        pdc.setCutter(&cutter);
        pdc.setSampling(sampling);
        pdc.setZ(min_z);

        std::vector<double> xyz;
        std::vector<int> counts;
        counts.reserve(nseg);
        for (int s = 0; s < nseg; ++s) {
            const double *g = segs + 4 * s;
            Path path;
            path.append(Line(Point(g[0], g[1], min_z), Point(g[2], g[3], min_z)));
            pdc.setPath(&path);
            pdc.run();
            std::vector<CLPoint> pts = pdc.getPoints();
            counts.push_back(static_cast<int>(pts.size()));
            for (const CLPoint &p : pts) {
                xyz.push_back(p.x);
                xyz.push_back(p.y);
                xyz.push_back(p.z);
            }
        }

        int total = static_cast<int>(xyz.size() / 3);
        double *ox = static_cast<double *>(std::malloc(xyz.empty() ? sizeof(double) : xyz.size() * sizeof(double)));
        int *oc = static_cast<int *>(std::malloc(counts.size() * sizeof(int)));
        if (!ox || !oc) {
            std::free(ox);
            std::free(oc);
            return -1;
        }
        if (!xyz.empty()) {
            std::memcpy(ox, xyz.data(), xyz.size() * sizeof(double));
        }
        std::memcpy(oc, counts.data(), counts.size() * sizeof(int));
        *out_xyz = ox;
        *out_counts = oc;
        return total;
    } catch (...) {
        return -1;
    }
}

extern "C" void obk_ocl_free_d(double *p) { std::free(p); }
extern "C" void obk_ocl_free_i(int *p) { std::free(p); }
