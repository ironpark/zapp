//go:build !windows

package macpkg

import (
	"fmt"
	"io/fs"
	"syscall"
)

func payloadMetadata(path string, info fs.FileInfo, ownership Ownership) (*syscall.Stat_t, error) {
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("unsupported file metadata for %s", path)
	}
	return s, nil
}
