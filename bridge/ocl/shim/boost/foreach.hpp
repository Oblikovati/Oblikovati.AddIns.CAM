// Minimal BOOST_FOREACH shim → C++11 range-for, so OpenCAMLib's drop-cutter
// compiles without the real Boost dependency.
#ifndef OCL_SHIM_BOOST_FOREACH_HPP
#define OCL_SHIM_BOOST_FOREACH_HPP
#define BOOST_FOREACH(decl, col) for (decl : col)
#endif
