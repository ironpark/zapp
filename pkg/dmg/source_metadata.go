package dmg

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

// Imported Finder metadata is part of the content tree, independently of the
// filesystem selected for the output. Do not follow framework symlinks.
func sourceMetadata(path string, node *imageNode) error {
	absent := func(err error) bool {
		return errors.Is(err, errNoAttr) || errors.Is(err, unix.ENOTSUP)
	}
	if _, err := getSourceXattr(path, finderInfoAttr, node.FinderInfo[:]); err != nil && !absent(err) {
		return fmt.Errorf("read FinderInfo of %s: %w", path, err)
	}
	size, err := getSourceXattr(path, resourceForkAttr, nil)
	if absent(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read resource fork of %s: %w", path, err)
	}
	if size != 0 {
		node.ResourceFork, err = sourceResourceFork(path, size)
	}
	return err
}
