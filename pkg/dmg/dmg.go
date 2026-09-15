// Package dmg builds macOS disk images, including the Finder window layout the
// Finder shows when one is opened. The image is assembled whole rather than
// created and then mounted to be decorated, so building a DMG needs neither
// Apple's tools nor a mounted volume, and works off macOS.
package dmg

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ironpark/zapp/pkg/alias"
	"github.com/ironpark/zapp/pkg/dsstore"
	"github.com/ironpark/zapp/pkg/macfs"
	"github.com/ironpark/zapp/pkg/udif"
)

// Config describes the disk image and the Finder window that opens with it.
type Config struct {
	FileName         string `json:"fileName"`
	Title            string `json:"title"`
	Icon             string `json:"icon"`
	LabelSize        int    `json:"labelSize"`
	ContentsIconSize int    `json:"iconSize"`
	WindowWidth      int    `json:"windowWidth"`
	WindowHeight     int    `json:"windowHeight"`
	Background       string `json:"background"`
	Contents         []Item `json:"contents"`
	LogWriter        io.Writer

	// Format selects how the image is compressed. The zero value is zlib,
	// which every version of macOS can read.
	Format udif.Format

	// FileSystem selects the volume format. The zero value is HFS+.
	FileSystem FileSystem `json:"filesystem"`

	// Created is the timestamp recorded throughout the image. It defaults to
	// the current time; setting it makes a build reproducible.
	Created time.Time
}

type ItemType string

const (
	Dir  ItemType = "dir"
	File ItemType = "file"
	Link ItemType = "link"
)

// Item is one entry placed in the disk image's window. Path names what to put
// in the image, except for a link, where it is the target the link points at.
type Item struct {
	X    int      `json:"x"`
	Y    int      `json:"y"`
	Type ItemType `json:"type"`
	Path string   `json:"path"`
}

// Names of the entries the Finder reads a window's appearance from.
const (
	storeName      = ".DS_Store"
	backgroundDir  = ".background"
	backgroundName = "background.png"
	volumeIconName = ".VolumeIcon.icns"
)

// CreateDMG builds the disk image described by config.
func CreateDMG(ctx context.Context, config Config) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	fileSystem, err := ParseFileSystem(string(config.FileSystem))
	if err != nil {
		return err
	}
	config.FileSystem = fileSystem
	if config.Title == "" {
		return fmt.Errorf("a volume title is required")
	}
	if config.FileName == "" {
		config.FileName = config.Title + ".dmg"
	}
	if !strings.HasSuffix(config.FileName, ".dmg") {
		config.FileName += ".dmg"
	}
	if config.Created.IsZero() {
		config.Created = time.Now()
	}

	volume, err := config.buildVolume()
	if err != nil {
		return err
	}
	image, err := config.planImage(ctx, volume)
	if err != nil {
		return err
	}
	if err := config.writeImage(ctx, image); err != nil {
		return err
	}

	return nil
}

// writeImage streams the volume through the compressor into the output file.
// The image is planned first so its size is known, which lets the two stages
// run against each other rather than through a copy of the whole volume on disk.
func (c Config) writeImage(ctx context.Context, image plannedImage) error {
	output := c.FileName
	temp, err := os.CreateTemp(filepath.Dir(output), ".dmg-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(temp.Name()) }()
	defer func() { _ = temp.Close() }()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	reader, writer := io.Pipe()
	defer reader.Close()
	written := make(chan error, 1)
	go func() {
		_, err := image.WriteTo(ctx, writer)
		_ = writer.CloseWithError(err)
		written <- err
	}()
	if _, err = udif.WriteWithOptions(ctx, temp, reader, image.Size(), udif.Options{Format: c.Format, DiskType: c.FileSystem.diskType()}); err != nil {
		_ = reader.CloseWithError(err)
		return fmt.Errorf("failed to compress the disk image: %w", err)
	}
	if err := <-written; err != nil {
		return fmt.Errorf("failed to write the disk image: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.Icon != "" {
		if err := setFileIcon(temp.Name(), c.Icon); err != nil {
			return err
		}
	}
	// A temporary file is created private to its owner, but the image is
	// something to hand out.
	if err = temp.Chmod(0644); err != nil {
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	// Existing output survives a failed build, and appears complete or not at all.
	return os.Rename(temp.Name(), output)
}

// buildVolume turns the configured contents into the tree the image is written
// from, including the hidden entries the Finder reads a window's look from.
func (c Config) buildVolume() (*volumeTree, error) {
	root := &imageNode{Mode: fs.ModeDir | 0755, ModTime: c.Created}
	for _, item := range c.Contents {
		node, err := c.nodeFor(item)
		if err != nil {
			return nil, err
		}
		root.Children = append(root.Children, node)
	}

	if c.Background != "" {
		image, err := macfs.FromFile(c.Background)
		if err != nil {
			return nil, fmt.Errorf("failed to read the background image: %w", err)
		}
		root.Children = append(root.Children, &imageNode{
			Name: backgroundDir, Mode: fs.ModeDir | 0755, ModTime: c.Created,
			Children: []*imageNode{{Name: backgroundName, Mode: 0644, ModTime: c.Created, Data: image}},
		})
	}

	if c.Icon != "" {
		icon, err := macfs.FromFile(c.Icon)
		if err != nil {
			return nil, fmt.Errorf("failed to read the volume icon: %w", err)
		}
		// A volume takes its icon from a file at its root rather than from a
		// resource fork, and the Finder only looks for that file when the root
		// is flagged as having a custom icon.
		root.Children = append(root.Children, &imageNode{
			Name: volumeIconName, Mode: 0644, ModTime: c.Created, Data: icon,
			FinderInfo: finderInfoFor(volumeIconCreator, false),
		})
		root.FinderInfo = finderInfoFor("", true)
	}

	// A placeholder, replaced once the tree has been numbered and the window
	// settings can be encoded.
	root.Children = append(root.Children, &imageNode{Name: storeName, Mode: 0644, ModTime: c.Created, Data: macfs.Bytes(nil)})

	return &volumeTree{Name: c.Title, Created: c.Created, Root: root}, nil
}

// nodeFor turns one configured item into a tree node.
func (c Config) nodeFor(item Item) (*imageNode, error) {
	switch item.Type {
	case Link:
		return &imageNode{Name: filepath.Base(item.Path), Mode: fs.ModeSymlink, ModTime: c.Created, LinkTarget: item.Path}, nil
	case File, Dir:
		node, err := nodeFromPath(item.Path)
		if err != nil {
			return nil, err
		}
		if item.Type == Dir && !node.IsDir() {
			return nil, fmt.Errorf("%s is not a directory", item.Path)
		}
		return node, nil
	default:
		return nil, fmt.Errorf("unknown content type %q for %s", item.Type, item.Path)
	}
}

// nodeFromPath reads one filesystem entry into a tree node, recursing into
// directories. Symbolic links are kept as links rather than followed, which
// matters for an application bundle: a framework inside one is a web of links
// whose shape its code signature covers.
func nodeFromPath(path string) (*imageNode, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	return nodeFromInfo(path, info.Name(), info)
}

// nodeFromInfo builds the node for path, whose metadata has already been read.
// name is the entry's name in its parent, which differs from info.Name() only
// for a directory named on the command line.
func nodeFromInfo(path, name string, info fs.FileInfo) (*imageNode, error) {
	node := &imageNode{Name: name, Mode: info.Mode(), ModTime: info.ModTime()}
	switch mode := info.Mode(); {
	case mode&fs.ModeSymlink != 0:
		// A link carries no Finder metadata of its own, and reading its
		// extended attributes would follow it to its target.
		target, err := os.Readlink(path)
		if err != nil {
			return nil, err
		}
		node.Mode, node.LinkTarget = fs.ModeSymlink, target
		return node, nil
	case mode.IsDir():
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			entryInfo, err := e.Info()
			if err != nil {
				return nil, err
			}
			child, err := nodeFromInfo(filepath.Join(path, e.Name()), e.Name(), entryInfo)
			if err != nil {
				return nil, err
			}
			node.Children = append(node.Children, child)
		}
	case mode.IsRegular():
		node.Data = macfs.File(path, info.Size())
	default:
		return nil, fmt.Errorf("%s is neither a file, a directory, nor a link", path)
	}
	if err := sourceMetadata(path, node); err != nil {
		return nil, err
	}
	return node, nil
}

// buildStore encodes the window settings the Finder reads when the image is
// opened: its size, how its icons are drawn, and where each one sits.
func (c Config) buildStore(volume *volumeTree) ([]byte, error) {
	store := dsstore.NewDSStore()
	store.SetIconSize(float64(c.ContentsIconSize))
	store.SetWindow(c.WindowWidth, c.WindowHeight, 0, 0)
	store.SetLabelSize(float64(c.LabelSize))
	store.SetLabelPlaceToBottom(true)
	store.SetBgToDefault()
	for _, item := range c.Contents {
		store.SetIconPos(filepath.Base(item.Path), uint32(item.X), uint32(item.Y))
	}

	if c.Background != "" {
		record, err := c.backgroundAlias(volume)
		if err != nil {
			return nil, fmt.Errorf("failed to build the background image alias: %w", err)
		}
		store.SetBackgroundImage(record)
	}

	return store.Encode(), nil
}

// backgroundAlias encodes the record naming the background image. The record
// identifies the file by its catalog node ID, which on a mounted volume is what
// stat reports as its inode; because the image is numbered before it is
// written, those IDs are known here without ever mounting it.
func (c Config) backgroundAlias(volume *volumeTree) ([]byte, error) {
	parent := volume.Root.Child(backgroundDir)
	if parent == nil {
		return nil, fmt.Errorf("the background directory is missing from the image")
	}
	image := parent.Child(backgroundName)
	if image == nil {
		return nil, fmt.Errorf("the background image is missing from the image")
	}
	if image.ID > 0xffffffff || parent.ID > 0xffffffff {
		return nil, fmt.Errorf("background alias identifier exceeds 32 bits")
	}
	target := alias.Target{
		Path:          path.Join("/", backgroundDir, backgroundName),
		ID:            uint32(image.ID),
		ParentID:      uint32(parent.ID),
		Created:       c.Created,
		VolumeName:    volume.Name,
		VolumeCreated: volume.Created,
		Identity:      c.FileSystem.aliasVolume(),
	}
	return alias.Create(target)
}

// setFileIcon gives the disk image itself a custom Finder icon.
func setFileIcon(dmgPath, iconPath string) error {
	icns, err := os.ReadFile(iconPath)
	if err != nil {
		return fmt.Errorf("failed to read icon %s: %w", iconPath, err)
	}
	if err := applyImageIcon(dmgPath, icns); err != nil {
		return fmt.Errorf("failed to set icon on %s: %w", dmgPath, err)
	}
	return nil
}
