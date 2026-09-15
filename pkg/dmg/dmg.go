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
	"github.com/ironpark/zapp/pkg/hfsplus"
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
	// Numbering the tree first is what lets the window settings be written into
	// the image: the alias record naming the background image has to carry the
	// catalog node IDs the volume will report once it is mounted.
	if err := hfsplus.AssignIDs(volume); err != nil {
		return err
	}
	store, err := config.buildStore(volume)
	if err != nil {
		return err
	}
	setChild(volume.Root, storeName, hfsplus.Bytes(store))

	if err := writeImage(ctx, config.FileName, *volume); err != nil {
		return err
	}
	if config.Icon != "" {
		// The image file itself carries an icon the way any other file does,
		// through a resource fork on whatever filesystem it is sitting on.
		if err := setFileIcon(config.FileName, config.Icon); err != nil {
			return err
		}
	}
	return nil
}

// writeImage streams the volume through the compressor into the output file.
// The image is planned first so its size is known, which lets the two stages
// run against each other rather than through a copy of the whole volume on disk.
func writeImage(ctx context.Context, output string, volume hfsplus.Volume) error {
	image, err := hfsplus.Plan(ctx, volume)
	if err != nil {
		return fmt.Errorf("failed to lay out the disk image: %w", err)
	}

	temp, err := os.CreateTemp(filepath.Dir(output), ".dmg-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(temp.Name()) }()
	defer func() { _ = temp.Close() }()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	reader, writer := io.Pipe()
	go func() {
		_, err := image.WriteTo(ctx, writer)
		_ = writer.CloseWithError(err)
	}()
	if _, err = udif.Write(ctx, temp, reader, image.Size()); err != nil {
		_ = reader.CloseWithError(err)
		return fmt.Errorf("failed to compress the disk image: %w", err)
	}
	if err = temp.Close(); err != nil {
		return err
	}
	// Existing output survives a failed build, and appears complete or not at all.
	return os.Rename(temp.Name(), output)
}

// buildVolume turns the configured contents into the tree the image is written
// from, including the hidden entries the Finder reads a window's look from.
func (c Config) buildVolume() (*hfsplus.Volume, error) {
	root := &hfsplus.Node{Mode: fs.ModeDir, ModTime: c.Created}
	for _, item := range c.Contents {
		node, err := c.nodeFor(item)
		if err != nil {
			return nil, err
		}
		root.Children = append(root.Children, node)
	}

	if c.Background != "" {
		image, err := hfsplus.FromFile(c.Background)
		if err != nil {
			return nil, fmt.Errorf("failed to read the background image: %w", err)
		}
		root.Children = append(root.Children, &hfsplus.Node{
			Name: backgroundDir, Mode: fs.ModeDir, ModTime: c.Created,
			Children: []*hfsplus.Node{{Name: backgroundName, ModTime: c.Created, Data: image}},
		})
	}

	if c.Icon != "" {
		icon, err := hfsplus.FromFile(c.Icon)
		if err != nil {
			return nil, fmt.Errorf("failed to read the volume icon: %w", err)
		}
		// A volume takes its icon from a file at its root rather than from a
		// resource fork, and the Finder only looks for that file when the root
		// is flagged as having a custom icon.
		root.Children = append(root.Children, &hfsplus.Node{
			Name: volumeIconName, ModTime: c.Created, Data: icon,
			FinderInfo: finderInfoFor(volumeIconCreator, false),
		})
		root.FinderInfo = finderInfoFor("", true)
	}

	// A placeholder, replaced once the tree has been numbered and the window
	// settings can be encoded.
	root.Children = append(root.Children, &hfsplus.Node{Name: storeName, ModTime: c.Created, Data: hfsplus.Bytes(nil)})

	return &hfsplus.Volume{Name: c.Title, Created: c.Created, Root: root}, nil
}

// nodeFor turns one configured item into a tree node.
func (c Config) nodeFor(item Item) (*hfsplus.Node, error) {
	name := filepath.Base(item.Path)
	switch item.Type {
	case Link:
		return &hfsplus.Node{Name: name, Mode: fs.ModeSymlink, ModTime: c.Created, LinkTarget: item.Path}, nil
	case File:
		data, err := hfsplus.FromFile(item.Path)
		if err != nil {
			return nil, err
		}
		info, err := os.Lstat(item.Path)
		if err != nil {
			return nil, err
		}
		return &hfsplus.Node{Name: name, Mode: info.Mode(), ModTime: info.ModTime(), Data: data}, nil
	case Dir:
		return nodeFromDir(item.Path)
	default:
		return nil, fmt.Errorf("unknown content type %q for %s", item.Type, item.Path)
	}
}

// nodeFromDir reads a directory into a tree. Symbolic links are kept as links
// rather than followed, which matters for an application bundle: a framework
// inside one is a web of links whose shape its code signature covers.
func nodeFromDir(dir string) (*hfsplus.Node, error) {
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	node := &hfsplus.Node{Name: filepath.Base(dir), Mode: fs.ModeDir, ModTime: info.ModTime()}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		child := filepath.Join(dir, e.Name())
		entryInfo, err := e.Info()
		if err != nil {
			return nil, err
		}
		switch mode := entryInfo.Mode(); {
		case mode&fs.ModeSymlink != 0:
			target, err := os.Readlink(child)
			if err != nil {
				return nil, err
			}
			node.Children = append(node.Children, &hfsplus.Node{
				Name: e.Name(), Mode: fs.ModeSymlink, ModTime: entryInfo.ModTime(), LinkTarget: target,
			})
		case mode.IsDir():
			sub, err := nodeFromDir(child)
			if err != nil {
				return nil, err
			}
			node.Children = append(node.Children, sub)
		case mode.IsRegular():
			data, err := hfsplus.FromFile(child)
			if err != nil {
				return nil, err
			}
			node.Children = append(node.Children, &hfsplus.Node{
				Name: e.Name(), Mode: mode, ModTime: entryInfo.ModTime(), Data: data,
			})
		default:
			return nil, fmt.Errorf("%s is neither a file, a directory, nor a link", child)
		}
	}
	return node, nil
}

// setChild replaces the contents of a named entry at the root of the tree.
func setChild(root *hfsplus.Node, name string, data hfsplus.Source) {
	for _, child := range root.Children {
		if child.Name == name {
			child.Data = data
			return
		}
	}
}

// buildStore encodes the window settings the Finder reads when the image is
// opened: its size, how its icons are drawn, and where each one sits.
func (c Config) buildStore(volume *hfsplus.Volume) ([]byte, error) {
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
func (c Config) backgroundAlias(volume *hfsplus.Volume) ([]byte, error) {
	parent := findChild(volume.Root, backgroundDir)
	if parent == nil {
		return nil, fmt.Errorf("the background directory is missing from the image")
	}
	image := findChild(parent, backgroundName)
	if image == nil {
		return nil, fmt.Errorf("the background image is missing from the image")
	}
	return alias.Create(alias.Target{
		Path:          path.Join("/", backgroundDir, backgroundName),
		ID:            image.ID,
		ParentID:      parent.ID,
		Created:       c.Created,
		VolumeName:    volume.Name,
		VolumeCreated: volume.Created,
	})
}

func findChild(parent *hfsplus.Node, name string) *hfsplus.Node {
	for _, child := range parent.Children {
		if child.Name == name {
			return child
		}
	}
	return nil
}

// setFileIcon gives the disk image itself a custom Finder icon.
func setFileIcon(dmgPath, iconPath string) error {
	icns, err := os.ReadFile(iconPath)
	if err != nil {
		return fmt.Errorf("failed to read icon %s: %w", iconPath, err)
	}
	if err := applyCustomIcon(dmgPath, icns); err != nil {
		return fmt.Errorf("failed to set icon on %s: %w", dmgPath, err)
	}
	return nil
}
