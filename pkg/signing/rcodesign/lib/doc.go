// Package lib carries the prebuilt libcodesign static libraries that
// zapp's rcodesign binding links on Linux and Windows.
//
// The archives are a module of their own so that only the builds that link
// them download them: the binding imports this package under the same build
// constraint as its cgo code, cgo on linux or windows and amd64 or arm64, and
// a macOS or CGO_ENABLED=0 build of zapp never loads this module. Importing
// it links the archive for the target and the system libraries it needs; it
// declares no Go API. The C declarations are in the binding's
// zapp_rcodesign.h, which has to match the archives here.
//
// The archives come from github.com/ironpark/libcodesign, at the release
// zapp's scripts/libcodesign.json pins; scripts/fetch_libcodesign.py writes
// them here. LICENSE, NOTICE and licenses/ are libcodesign's notices, which
// travel with the archives.
package lib
