//go:build !windows

package macpkg

import (
	"fmt"
	"io/fs"
	"syscall"
)

// ownershipSupported reports whether the host can report Unix uid/gid.
const ownershipSupported = true

func payloadMetadata(path string, info fs.FileInfo) (*payloadStat, error) {
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("unsupported file metadata for %s", path)
	}
	return &payloadStat{
		Mode: uint32(s.Mode), Uid: s.Uid, Gid: s.Gid,
		Dev: uint64(s.Dev), Ino: uint64(s.Ino),
	}, nil
}
