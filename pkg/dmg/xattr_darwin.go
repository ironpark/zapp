package dmg

import (
	"errors"

	"golang.org/x/sys/unix"

	"github.com/ironpark/zapp/pkg/macfs"
)

// errNoAttr is what the kernel reports for an attribute that is not set.
var errNoAttr = unix.ENOATTR

// absentAttr reports whether err means the attribute is simply not there.
func absentAttr(err error) bool {
	return errors.Is(err, errNoAttr) || errors.Is(err, unix.ENOTSUP)
}

func applyImageIcon(path string, icns []byte) error { return applyCustomIcon(path, icns) }

func getXattr(path, name string, buf []byte) (int, error) {
	return unix.Getxattr(path, name, buf)
}

func getSourceXattr(path, name string, buf []byte) (int, error) {
	return unix.Lgetxattr(path, name, buf)
}

func sourceResourceFork(path string, size int) (macfs.Source, error) {
	return macfs.File(path+"/..namedfork/rsrc", int64(size)), nil
}

func setXattr(path, name string, data []byte) error {
	return unix.Setxattr(path, name, data, 0)
}
