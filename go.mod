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

go 1.27.0

require oblikovati.org/api v0.153.1

require (
	github.com/bitfield/gotestdox v0.2.2 // indirect
	github.com/dnephin/pflag v1.0.7 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	golang.org/x/mod v0.27.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	golang.org/x/term v0.35.0 // indirect
	golang.org/x/text v0.17.0 // indirect
	golang.org/x/tools v0.36.0 // indirect
	gotest.tools/gotestsum v1.13.0 // indirect
)

tool gotest.tools/gotestsum
