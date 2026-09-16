//go:build !unix

package hdiutil

// hdiutil ships with macOS alone, so nothing away from unix ever runs it and
// there is nothing to serialize.
func lock() (func(), error) { return func() {}, nil }
