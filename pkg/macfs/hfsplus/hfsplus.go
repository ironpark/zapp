// Package hfsplus writes HFS+ filesystem images in Go, without CGO, Apple
// command-line tools, or a mounted volume. Images are built whole from a tree
// held in memory, so every catalog node ID is known before any byte is written
// and metadata that normally requires mounting the volume can be prepared up
// front. Reading images back is not supported.
package hfsplus

import (
	"fmt"
	"math"
	"time"

	"github.com/ironpark/zapp/pkg/macfs"
)

// The tree written to an image is the shared one from pkg/macfs; these
// aliases let callers name it without importing both packages.
type (
	Source = macfs.Source
	Node   = macfs.Node
)

// Bytes returns a Source holding b. The slice must not be modified afterwards.
func Bytes(b []byte) Source { return macfs.Bytes(b) }

// FromFile returns a Source reading path, whose size is fixed at this call.
// The file must not change until the image has been written.
func FromFile(path string) (Source, error) { return macfs.FromFile(path) }

// Volume describes an image to write.
type Volume struct {
	Name    string // Volume name, as the Finder displays it.
	Created time.Time
	Root    *Node // Must be a directory. Its Name is unused.

	// BlockSize is the allocation block size, defaulting to 4096. It must be a
	// power of two of at least 512.
	BlockSize uint32

	// FreeSpace reserves this many bytes beyond the contents, letting an image
	// intended to be written to still hold something.
	FreeSpace int64
}

// Catalog node IDs reserved by the format. User entries are numbered upwards
// from firstUserID.
const (
	rootParentID = 1
	rootFolderID = 2
	extentsID    = 3
	catalogID    = 4
	badBlockID   = 5
	allocationID = 6
	startupID    = 7
	attributesID = 8
	firstUserID  = 16
)

// AssignIDs numbers every node in the tree, so that metadata referring to an
// entry by ID can be built before the image exists. Write calls it when the
// root has not been numbered yet; call it first when the contents of one node
// depend on the ID of another, as a .DS_Store holding an alias record does.
// Numbering depends only on the shape of the tree and the entry names, not on
// file contents, so contents may still be changed afterwards.
func AssignIDs(v *Volume) error {
	if v.Root == nil || !v.Root.IsDir() {
		return fmt.Errorf("volume root must be a directory")
	}
	v.Root.ID = rootFolderID
	next := uint64(firstUserID)
	var walk func(*Node) error
	walk = func(dir *Node) error {
		sorted, err := sortChildren(dir.Children)
		if err != nil {
			return err
		}
		dir.Children = sorted
		for _, child := range dir.Children {
			if next > math.MaxUint32 {
				return fmt.Errorf("too many entries for an HFS+ catalog")
			}
			child.ID = next
			next++
		}
		for _, child := range dir.Children {
			if child.IsDir() {
				if err := walk(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(v.Root)
}
