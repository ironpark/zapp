package dmg

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/ironpark/zapp/pkg/macfs/apfs"
	"github.com/ironpark/zapp/pkg/macfs/hfsplus"
	"github.com/ironpark/zapp/pkg/udif"
)

// FileSystem selects the disk image's filesystem independently of compression.
type FileSystem string

const (
	HFSPlus           FileSystem = "hfsplus"
	APFS              FileSystem = "apfs"
	APFSCaseSensitive FileSystem = "apfs-case-sensitive"
)

func (f FileSystem) String() string {
	if f == "" {
		return string(HFSPlus)
	}
	return string(f)
}
func (f FileSystem) valid() bool {
	return f == "" || f == HFSPlus || f == APFS || f == APFSCaseSensitive
}
func (f FileSystem) diskType() udif.DiskType {
	if f == APFS || f == APFSCaseSensitive {
		return udif.AppleAPFS
	}
	return udif.AppleHFS
}

// Collection and Finder layout use a filesystem-neutral tree. Only adapters
// below know about each writer's node type or numbering rules.
type imageSource interface {
	Size() int64
	Open() (io.ReadCloser, error)
}
type imageNode struct {
	Name               string
	Mode               fs.FileMode
	ModTime            time.Time
	FinderInfo         [32]byte
	Data, ResourceFork imageSource
	LinkTarget         string
	Children           []*imageNode
	ID                 uint64
}
type volumeTree struct {
	Name    string
	Created time.Time
	Root    *imageNode
}
type plannedImage interface {
	Size() int64
	WriteTo(context.Context, io.Writer) (int64, error)
}

func (c Config) planImage(ctx context.Context, v *volumeTree) (plannedImage, error) {
	if c.FileSystem == "" || c.FileSystem == HFSPlus {
		mapping := map[*imageNode]*hfsplus.Node{}
		var convert func(*imageNode) *hfsplus.Node
		convert = func(n *imageNode) *hfsplus.Node {
			out := &hfsplus.Node{Name: n.Name, Mode: n.Mode, ModTime: n.ModTime, FinderInfo: n.FinderInfo, Data: n.Data, ResourceFork: n.ResourceFork, LinkTarget: n.LinkTarget}
			mapping[n] = out
			for _, child := range n.Children {
				out.Children = append(out.Children, convert(child))
			}
			return out
		}
		volume := hfsplus.Volume{Name: v.Name, Created: v.Created, Root: convert(v.Root)}
		if err := hfsplus.AssignIDs(&volume); err != nil {
			return nil, err
		}
		for n, out := range mapping {
			n.ID = uint64(out.ID)
		}
		store, err := c.buildStore(v)
		if err != nil {
			return nil, err
		}
		mapping[findChild(v.Root, storeName)].Data = hfsplus.Bytes(store)
		image, err := hfsplus.Plan(ctx, volume)
		if err != nil {
			return nil, fmt.Errorf("failed to lay out HFS+ image: %w", err)
		}
		return image, nil
	}
	mapping := map[*imageNode]*apfs.Node{}
	var convert func(*imageNode) *apfs.Node
	convert = func(n *imageNode) *apfs.Node {
		out := &apfs.Node{Name: n.Name, Mode: n.Mode, ModTime: n.ModTime, FinderInfo: n.FinderInfo, Data: n.Data, ResourceFork: n.ResourceFork, LinkTarget: n.LinkTarget}
		mapping[n] = out
		for _, child := range n.Children {
			out.Children = append(out.Children, convert(child))
		}
		return out
	}
	volume := apfs.Volume{Name: v.Name, Created: v.Created, Root: convert(v.Root), CaseSensitive: c.FileSystem == APFSCaseSensitive}
	if err := apfs.AssignIDs(&volume); err != nil {
		return nil, err
	}
	for n, out := range mapping {
		n.ID = out.ID
	}
	store, err := c.buildStore(v)
	if err != nil {
		return nil, err
	}
	mapping[findChild(v.Root, storeName)].Data = apfs.Bytes(store)
	image, err := apfs.Plan(ctx, volume)
	if err != nil {
		return nil, fmt.Errorf("failed to lay out APFS image: %w", err)
	}
	return image, nil
}

// Data sources are structural interfaces accepted by either filesystem writer.
type imageBytes []byte

func (b imageBytes) Size() int64                  { return int64(len(b)) }
func (b imageBytes) Open() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(b)), nil }

type imageFile struct {
	path string
	size int64
}

func (f imageFile) Size() int64                  { return f.size }
func (f imageFile) Open() (io.ReadCloser, error) { return os.Open(f.path) }
func imageFromFile(path string) (imageSource, error) {
	i, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !i.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	return imageFile{path, i.Size()}, nil
}
