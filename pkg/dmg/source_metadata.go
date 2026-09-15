package dmg

import "fmt"

// Imported Finder metadata is part of the content tree, independently of the
// filesystem selected for the output. Do not follow framework symlinks.
func sourceMetadata(path string, node *imageNode) error {
	if _, err := getSourceXattr(path, finderInfoAttr, node.FinderInfo[:]); err != nil && !absentAttr(err) {
		return fmt.Errorf("read FinderInfo of %s: %w", path, err)
	}
	size, err := getSourceXattr(path, resourceForkAttr, nil)
	if absentAttr(err) {
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
