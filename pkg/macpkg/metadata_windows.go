package macpkg

import (
	"fmt"
	"golang.org/x/sys/windows"
	"io/fs"
)

type windowsPayloadMetadata struct {
	Mode, Uid, Gid uint32
	Dev, Ino       uint64
}

func payloadMetadata(path string, info fs.FileInfo, ownership Ownership) (*windowsPayloadMetadata, error) {
	if ownership == PreserveOwnership {
		return nil, fmt.Errorf("preserving Unix ownership is not supported on Windows: %s", path)
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
	mode := uint32(info.Mode().Perm()) | 0100000
	if info.IsDir() {
		mode = 0040755
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		mode = 0120777
	}
	return &windowsPayloadMetadata{Mode: mode, Dev: uint64(data.VolumeSerialNumber), Ino: uint64(data.FileIndexHigh)<<32 | uint64(data.FileIndexLow)}, nil
}
