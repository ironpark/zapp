package dmg

import (
	"errors"

	"golang.org/x/sys/unix"

	"github.com/ironpark/zapp/pkg/macfs"
)

// Linux keeps extended attributes written by unprivileged processes in the
// "user." namespace, so the names here carry a prefix the same attribute lacks
// on macOS.
const linuxAttrPrefix = "user."

// errNoAttr is what the kernel reports for an attribute that is not set. Linux
// spells it ENODATA where macOS spells it ENOATTR.
var errNoAttr = unix.ENODATA

// absentAttr reports whether err means the attribute is simply not there.
func absentAttr(err error) bool {
	return errors.Is(err, errNoAttr) || errors.Is(err, unix.ENOTSUP)
}

func applyImageIcon(path string, icns []byte) error {
	err := applyCustomIcon(path, icns)
	// The volume's full icon is already inside the DMG. Linux may not be
	// able to attach that same icon to the host file: xattrs have a 64 KiB
	// ceiling, and some filesystems do not support them at all.
	if errors.Is(err, unix.E2BIG) || errors.Is(err, unix.ENOTSUP) {
		return nil
	}
	return err
}

func getXattr(path, name string, buf []byte) (int, error) {
	return unix.Getxattr(path, linuxAttrPrefix+name, buf)
}

func getSourceXattr(path, name string, buf []byte) (int, error) {
	return unix.Lgetxattr(path, linuxAttrPrefix+name, buf)
}

func sourceResourceFork(path string, size int) (macfs.Source, error) {
	// Linux xattrs are bounded by the filesystem's small attribute limit;
	// unlike macOS named forks, they cannot be opened as a byte stream.
	b := make([]byte, size)
	n, err := getSourceXattr(path, resourceForkAttr, b)
	if err != nil {
		return nil, err
	}
	return macfs.Bytes(b[:n]), nil
}

func setXattr(path, name string, data []byte) error {
	return unix.Setxattr(path, linuxAttrPrefix+name, data, 0)
}
