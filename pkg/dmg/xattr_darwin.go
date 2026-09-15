package dmg

import "golang.org/x/sys/unix"

// errNoAttr is what the kernel reports for an attribute that is not set.
var errNoAttr = unix.ENOATTR

func getXattr(path, name string, buf []byte) (int, error) {
	return unix.Getxattr(path, name, buf)
}

func setXattr(path, name string, data []byte) error {
	return unix.Setxattr(path, name, data, 0)
}
