package dmg

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ironpark/zapp/pkg/alias"

	"github.com/ironpark/zapp/pkg/macfs"

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

func (f FileSystem) String() string { return string(f.orDefault()) }

// orDefault resolves the zero value, so nothing downstream compares against "".
func (f FileSystem) orDefault() FileSystem {
	if f == "" {
		return HFSPlus
	}
	return f
}

// ParseFileSystem resolves a filesystem name, ignoring case. It is the single
// place that knows which names are accepted.
func ParseFileSystem(name string) (FileSystem, error) {
	switch f := FileSystem(strings.ToLower(name)); f {
	case "", HFSPlus, APFS, APFSCaseSensitive:
		return f.orDefault(), nil
	default:
		return "", fmt.Errorf("unknown filesystem %q: use %s, %s, or %s", name, HFSPlus, APFS, APFSCaseSensitive)
	}
}

func (f FileSystem) isAPFS() bool {
	return f.orDefault() != HFSPlus
}

func (f FileSystem) diskType() udif.DiskType {
	if f.isAPFS() {
		return udif.AppleAPFS
	}
	return udif.AppleHFS
}

// aliasVolume describes the volume the way FSNewAlias does on a mounted image
// of this filesystem. The APFS values, including the "network" classification,
// are what the legacy call reports; no network is involved.
func (f FileSystem) aliasVolume() alias.VolumeIdentity {
	if f.isAPFS() {
		return alias.VolumeIdentity{Signature: "BD", FSID: 0x6375, Attributes: 0x00000e02, Type: "network"}
	}
	return alias.VolumeIdentity{Signature: "H+", Attributes: 0x00000d02, Type: "other"}
}

// Collection and Finder layout use the shared macfs tree, so a tree can be
// numbered by one writer and laid out by another without conversion.
type imageNode = macfs.Node

type volumeTree struct {
	Name    string
	Created time.Time
	Root    *imageNode
}
type plannedImage interface {
	Size() int64
	WriteTo(context.Context, io.Writer) (int64, error)
}

// planImage numbers the tree, encodes the window settings now that every ID is
// known, and lays the volume out.
func (c Config) planImage(ctx context.Context, v *volumeTree) (plannedImage, error) {
	hfs := !c.FileSystem.isAPFS()
	if hfs {
		if err := hfsplus.AssignIDs(&hfsplus.Volume{Name: v.Name, Created: v.Created, Root: v.Root}); err != nil {
			return nil, err
		}
	} else if err := apfs.AssignIDs(&apfs.Volume{Name: v.Name, Created: v.Created, Root: v.Root}); err != nil {
		return nil, err
	}
	store, err := c.buildStore(v)
	if err != nil {
		return nil, err
	}
	v.Root.Child(storeName).Data = macfs.Bytes(store)

	if hfs {
		image, err := hfsplus.Plan(ctx, hfsplus.Volume{Name: v.Name, Created: v.Created, Root: v.Root})
		if err != nil {
			return nil, fmt.Errorf("failed to lay out HFS+ image: %w", err)
		}
		return image, nil
	}
	image, err := apfs.Plan(ctx, apfs.Volume{
		Name: v.Name, Created: v.Created, Root: v.Root,
		CaseSensitive: c.FileSystem == APFSCaseSensitive,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to lay out APFS image: %w", err)
	}
	return image, nil
}
