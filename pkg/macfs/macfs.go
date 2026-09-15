// Package macfs holds the pieces shared by the filesystem writers beneath it:
// the tree of entries an image is built from, and the sources their contents
// are read from. Nothing here depends on a particular on-disk format, so a
// tree can be numbered by one writer and laid out by another.
package macfs

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
	return File(path, info.Size()), nil
}

// File returns a Source reading path, trusting the caller's size. It saves a
// stat when the caller has already read the entry's metadata.
func File(path string, size int64) Source { return fileSource{path: path, size: size} }

type fileSource struct {
	path string
	size int64
}

func (f fileSource) Size() int64 { return f.size }

func (f fileSource) Open() (io.ReadCloser, error) { return os.Open(f.path) }

// Node is one directory, regular file, or symbolic link in an image. Mode
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
	// entry's inode. Each writer's AssignIDs fills it in.
	ID uint64
}

// IsDir reports whether n is a directory.
func (n *Node) IsDir() bool { return n.Mode&fs.ModeDir != 0 }

// IsSymlink reports whether n is a symbolic link.
func (n *Node) IsSymlink() bool { return n.Mode&fs.ModeSymlink != 0 }

// Child returns the child of n named name, or nil if there is none.
func (n *Node) Child(name string) *Node {
	for _, child := range n.Children {
		if child.Name == name {
			return child
		}
	}
	return nil
}
