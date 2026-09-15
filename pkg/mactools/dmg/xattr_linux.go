package dmg

import "golang.org/x/sys/unix"

// Linux keeps extended attributes written by unprivileged processes in the
// "user." namespace, so the names here carry a prefix the same attribute lacks
// on macOS.
const linuxAttrPrefix = "user."

// errNoAttr is what the kernel reports for an attribute that is not set. Linux
// spells it ENODATA where macOS spells it ENOATTR.
var errNoAttr = unix.ENODATA

func getXattr(path, name string, buf []byte) (int, error) {
	return unix.Getxattr(path, linuxAttrPrefix+name, buf)
}

func setXattr(path, name string, data []byte) error {
	return unix.Setxattr(path, linuxAttrPrefix+name, data, 0)
}
