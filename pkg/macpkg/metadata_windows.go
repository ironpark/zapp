package macpkg

import (
	"io/fs"

	"golang.org/x/sys/windows"
)

// ownershipSupported reports whether the host can report Unix uid/gid. Windows
// has no such notion, so collect rejects PreserveOwnership up front.
const ownershipSupported = false

func payloadMetadata(path string, info fs.FileInfo) (*payloadStat, error) {
	mode := uint32(info.Mode().Perm()) | 0100000
	switch {
	case info.IsDir():
		mode = 0040755
	case info.Mode()&fs.ModeSymlink != 0:
		mode = 0120777
	}
	// Dev and Ino identify hard links, which only regular files share. Opening
	// a handle is expensive on Windows, so skip it for everything else.
	if !info.Mode().IsRegular() {
		return &payloadStat{Mode: mode}, nil
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(handle)
	var data windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &data); err != nil {
		return nil, err
	}
	return &payloadStat{
		Mode: mode,
		Dev:  uint64(data.VolumeSerialNumber),
		Ino:  uint64(data.FileIndexHigh)<<32 | uint64(data.FileIndexLow),
	}, nil
}
