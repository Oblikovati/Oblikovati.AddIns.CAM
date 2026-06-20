// Shim: boost::math::isnan/isinf → std, so OpenCAMLib compiles without Boost.
#ifndef OCL_SHIM_BOOST_FPCLASSIFY_HPP
#define OCL_SHIM_BOOST_FPCLASSIFY_HPP
#include <cmath>
namespace boost { namespace math {
template <class T> inline bool isnan(T v) { return std::isnan(v); }
template <class T> inline bool isinf(T v) { return std::isinf(v); }
}}
#endif
