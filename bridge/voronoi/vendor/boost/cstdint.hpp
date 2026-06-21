// SPDX-License-Identifier: GPL-2.0-only

// Minimal boost/cstdint.hpp shim. The vendored Boost.Polygon Voronoi headers (BSL-1.0, see
// NOTICE.md) include <boost/cstdint.hpp> for four fixed-width integer typedefs. Rather than vendor
// the real Boost.Integer subtree, this maps those names onto the C++11 <cstdint> types, so the
// upstream Voronoi headers compile unmodified with no Boost dependency. This file is the project's,
// not Boost's.
#ifndef OBK_BOOST_CSTDINT_SHIM_H
#define OBK_BOOST_CSTDINT_SHIM_H

#include <cstdint>

namespace boost {
using int8_t = std::int8_t;
using int16_t = std::int16_t;
using int32_t = std::int32_t;
using int64_t = std::int64_t;
using uint8_t = std::uint8_t;
using uint16_t = std::uint16_t;
using uint32_t = std::uint32_t;
using uint64_t = std::uint64_t;
}  // namespace boost

#endif  // OBK_BOOST_CSTDINT_SHIM_H
