// Package hfsplus writes HFS+ filesystem images in Go, without CGO, Apple
// command-line tools, or a mounted volume. Images are built whole from a tree
// held in memory, so every catalog node ID is known before any byte is written
// and metadata that normally requires mounting the volume can be prepared up
// front. Reading images back is not supported.
package hfsplus

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"
)

// Source supplies the contents of one fork. Size must match what Open returns,
// and Open may be called more than once.
type Source interface {
	Size() int64
	Open() (io.ReadCloser, error)
}

// Bytes returns a Source holding b. The slice must not be modified afterwards.
func Bytes(b []byte) Source { return byteSource(b) }

type byteSource []byte

func (b byteSource) Size() int64 { return int64(len(b)) }

func (b byteSource) Open() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(b)), nil
}

// FromFile returns a Source reading path, whose size is fixed at this call.
// The file must not change until the image has been written.
func FromFile(path string) (Source, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	return fileSource{path: path, size: info.Size()}, nil
}

type fileSource struct {
	path string
	size int64
}

func (f fileSource) Size() int64 { return f.size }

func (f fileSource) Open() (io.ReadCloser, error) { return os.Open(f.path) }

// Node is one directory, regular file, or symbolic link in the image. Mode
// selects which: a node with fs.ModeDir set uses Children, one with
// fs.ModeSymlink uses LinkTarget, and any other node uses Data.
type Node struct {
	Name    string
	Mode    fs.FileMode
	ModTime time.Time

	// FinderInfo is the 32 bytes the Finder keeps for the entry, holding the
	// type and creator codes and flags such as kHasCustomIcon. It is stored in
	// the catalog record, the same place the com.apple.FinderInfo extended
	// attribute reports on a live volume.
	FinderInfo [32]byte

	Data         Source  // Contents of a regular file.
	ResourceFork Source  // Optional resource fork of a regular file.
	LinkTarget   string  // Target of a symbolic link.
	Children     []*Node // Contents of a directory.

	// ID is the catalog node ID, the number a live volume reports as the
	// entry's inode. AssignIDs fills it in.
	ID uint32
}

// IsDir reports whether n is a directory.
func (n *Node) IsDir() bool { return n.Mode&fs.ModeDir != 0 }

// IsSymlink reports whether n is a symbolic link.
func (n *Node) IsSymlink() bool { return n.Mode&fs.ModeSymlink != 0 }

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
	next := uint32(firstUserID)
	var walk func(*Node) error
	walk = func(dir *Node) error {
		sorted, err := sortChildren(dir.Children)
		if err != nil {
			return err
		}
		dir.Children = sorted
		for _, child := range dir.Children {
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
