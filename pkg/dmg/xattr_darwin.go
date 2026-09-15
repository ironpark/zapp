package dmg

import "golang.org/x/sys/unix"

// errNoAttr is what the kernel reports for an attribute that is not set.
var errNoAttr = unix.ENOATTR

func applyImageIcon(path string, icns []byte) error { return applyCustomIcon(path, icns) }

func getXattr(path, name string, buf []byte) (int, error) {
	return unix.Getxattr(path, name, buf)
}

func getSourceXattr(path, name string, buf []byte) (int, error) {
	return unix.Lgetxattr(path, name, buf)
}

func sourceResourceFork(path string, size int) (imageSource, error) {
	return imageFile{path: path + "/..namedfork/rsrc", size: int64(size)}, nil
}

func setXattr(path, name string, data []byte) error {
	return unix.Setxattr(path, name, data, 0)
}
