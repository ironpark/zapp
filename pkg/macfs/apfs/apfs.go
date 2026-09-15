// Package apfs writes single-volume, unencrypted APFS images without CGO,
// platform tools, or mounting. It implements the on-disk format described in
// Apple's Apple File System Reference (2020-06-22). It does not read images.
package apfs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Source supplies immutable file data. Open must return exactly Size bytes.
type Source interface {
	Size() int64
	Open() (io.ReadCloser, error)
}

type byteSource []byte

func (b byteSource) Size() int64                  { return int64(len(b)) }
func (b byteSource) Open() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(b)), nil }

// Bytes wraps b without copying it; do not modify b until writing has finished.
func Bytes(b []byte) Source { return byteSource(b) }

type fileSource struct {
	path string
	size int64
}

func (f fileSource) Size() int64                  { return f.size }
func (f fileSource) Open() (io.ReadCloser, error) { return os.Open(f.path) }

// FromFile captures the size of a regular file without reading its contents.
func FromFile(path string) (Source, error) {
	i, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !i.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	return fileSource{path, i.Size()}, nil
}

// Node is a directory, regular file, or symbolic link. ID is assigned before
// layout, allowing Finder metadata to refer to files before the image exists.
type Node struct {
	Name         string
	Mode         fs.FileMode
	ModTime      time.Time
	FinderInfo   [32]byte
	Data         Source
	ResourceFork Source
	LinkTarget   string
	Children     []*Node
	ID           uint64
}

func (n *Node) IsDir() bool     { return n.Mode.IsDir() }
func (n *Node) IsSymlink() bool { return n.Mode&fs.ModeSymlink != 0 }

// Volume describes a single APFS volume. Names are normalization insensitive
// in both modes. The default also ignores case, as macOS normally does.
type Volume struct {
	Name          string
	Created       time.Time
	Root          *Node
	CaseSensitive bool
}

func validName(name string) error {
	if name == "" || name == "." || name == ".." || !utf8.ValidString(name) || strings.ContainsAny(name, "\x00/") {
		return fmt.Errorf("invalid APFS name %q", name)
	}
	if len(name) > 255 {
		return fmt.Errorf("APFS name exceeds 255 UTF-8 bytes: %q", name)
	}
	return nil
}

// AssignIDs validates and deterministically numbers the tree. Data may be
// replaced afterwards, but the tree's shape and names must not change.
func AssignIDs(v *Volume) error { return assignIDs(context.Background(), v) }

func assignIDs(ctx context.Context, v *Volume) error {
	if v.Root == nil || !v.Root.IsDir() {
		return fmt.Errorf("volume root must be a directory")
	}
	next := uint64(16)
	seen := map[*Node]bool{}
	var walk func(*Node) error
	walk = func(n *Node) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if n == nil || seen[n] {
			return fmt.Errorf("volume must be a tree without nil or repeated nodes")
		}
		seen[n] = true
		if n != v.Root {
			if err := validName(n.Name); err != nil {
				return err
			}
			n.ID = next
			next++
		} else {
			n.ID = 2
		}
		if n.Mode.Type() != 0 && !n.IsDir() && !n.IsSymlink() {
			return fmt.Errorf("unsupported file type for %q", n.Name)
		}
		if !n.IsDir() && len(n.Children) != 0 {
			return fmt.Errorf("non-directory %q has children", n.Name)
		}
		if n.IsSymlink() && (n.LinkTarget == "" || strings.ContainsRune(n.LinkTarget, 0) || len(n.LinkTarget) >= 1024) {
			return fmt.Errorf("invalid symbolic link target for %q", n.Name)
		}
		names := map[string]bool{}
		for _, child := range n.Children {
			if child == nil {
				return fmt.Errorf("nil child in %q", n.Name)
			}
			key := normalizedName(child.Name, !v.CaseSensitive)
			if names[key] {
				return fmt.Errorf("duplicate APFS name %q in %q", child.Name, n.Name)
			}
			names[key] = true
		}
		sort.Slice(n.Children, func(i, j int) bool { return n.Children[i].Name < n.Children[j].Name })
		for _, child := range n.Children {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(v.Root)
}

// Image is an immutable layout; its file Sources must remain unchanged.
type Image struct{ layout *layout }

func (i *Image) Size() int64 { return int64(i.layout.total) * blockSize }
func (i *Image) WriteTo(ctx context.Context, w io.Writer) (int64, error) {
	return i.layout.write(ctx, w)
}

// Plan settles all metadata and data locations without reading file contents.
func Plan(ctx context.Context, v Volume) (*Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validName(v.Name); err != nil {
		return nil, fmt.Errorf("volume name: %w", err)
	}
	if v.Created.IsZero() {
		v.Created = time.Now()
	}
	if err := assignIDs(ctx, &v); err != nil {
		return nil, err
	}
	l, err := planVolume(ctx, v)
	if err != nil {
		return nil, err
	}
	return &Image{l}, nil
}

// Write plans and writes v, returning the number of bytes written.
func Write(ctx context.Context, w io.Writer, v Volume) (int64, error) {
	i, err := Plan(ctx, v)
	if err != nil {
		return 0, err
	}
	return i.WriteTo(ctx, w)
}
