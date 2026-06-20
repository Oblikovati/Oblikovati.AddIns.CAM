// The oblikovati-cam add-in: a c-shared library (.so/.dll/.dylib) loaded by the host
// at runtime, providing computer-aided manufacturing (machining Job → toolpaths →
// G-code). It pulls part geometry from the host over the Apache-2.0 API, generates
// toolpaths in-process, and posts them to machine G-code. Its own module so the CAM
// deps stay independent of the host — the runtime boundary is the C ABI, not Go (see
// include/oblikovati_addin.h).
//
// The SHIPPED library links only the Apache-2.0 contract (oblikovati.org/api). The
// require on the GPL application module (oblikovati) is TEST-SCOPE ONLY — the
// add-in↔real-host integration tests drive the live router/model. Both modules are
// sibling repos resolved by the go.work workspace at this repo's root (no committed
// replace); CI injects the equivalent replaces via .github/actions/siblings.
module oblikovati.org/cam

go 1.24.0

require oblikovati.org/api v0.78.0
