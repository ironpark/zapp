package dmg

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ironpark/zapp/internal/fsutil"
	"github.com/ironpark/zapp/pkg/dsstore"
	"github.com/ironpark/zapp/pkg/mactools/hdiutil"
)

// Config represents the configuration for the DMG file.
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
}

type ItemType string

const (
	Dir  ItemType = "dir"
	File ItemType = "file"
	Link ItemType = "link"
)

// Item represents an item in the DMG file.
type Item struct {
	X    int      `json:"x"`
	Y    int      `json:"y"`
	Type ItemType `json:"type"`
	Path string   `json:"path"`
}

// CreateDMG creates a DMG file with the specified configuration.
func CreateDMG(ctx context.Context, config Config, sourceDir string) error {
	// Create the source directory if it doesn't exist
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		return fmt.Errorf("failed to create source directory: %w", err)
	}
	// Setup the source directory with the necessary files
	if err := setupSourceDirectory(config, sourceDir); err != nil {
		return fmt.Errorf("failed to setup source directory: %w", err)
	}
	if config.LogWriter == nil {
		config.LogWriter = os.Stdout
	}
	store := dsstore.NewDSStore()
	store.SetIconSize(float64(config.ContentsIconSize))
	store.SetWindow(config.WindowWidth, config.WindowHeight, 0, 0)
	store.SetLabelSize(float64(config.LabelSize))
	store.SetLabelPlaceToBottom(true)
	store.SetBgToDefault()
	for _, content := range config.Contents {
		store.SetIconPos(filepath.Base(content.Path), uint32(content.X), uint32(content.Y))
	}
	err := store.Write(filepath.Join(sourceDir, ".DS_Store"))
	if err != nil {
		return fmt.Errorf("failed to write .DS_Store: %w", err)
	}

	// Set Default Filename
	if config.FileName == "" {
		config.FileName = config.Title + ".dmg"
	}

	if !strings.HasSuffix(config.FileName, ".dmg") {
		config.FileName += ".dmg"
	}
	// Create the DMG file using hdiutil
	if err := hdiutil.Create(ctx, config.Title, sourceDir, hdiutil.UDRW, config.FileName); err != nil {
		return fmt.Errorf("failed to create dmg: %w", err)
	}

	// Set custom icon for the DMG if specified
	if config.Icon != "" || config.Background != "" {
		err = tmpMount(ctx, config.FileName, func(dmgFilePath string, mountPoint string) error {
			if config.Icon != "" {
				if err := setDMGIcon(mountPoint, config.Icon); err != nil {
					return fmt.Errorf("failed to set DMG icon: %w", err)
				}
			}
			if config.Background != "" {
				// config.Title is the volume name handed to hdiutil, so it is
				// the name the alias record has to carry.
				bg := filepath.Join(mountPoint, ".background", "background.png")
				if err := store.SetBackgroundImage(bg, config.Title); err != nil {
					return fmt.Errorf("failed to build the background image alias: %w", err)
				}
				if err := store.Write(filepath.Join(mountPoint, ".DS_Store")); err != nil {
					return fmt.Errorf("failed to write .DS_Store: %w", err)
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to customize dmg appearance: %w", err)
		}
	}

	// Convert the DMG to read-only
	tempFileName := "temp_" + config.FileName
	dir, file := filepath.Split(config.FileName)
	tempFileName = filepath.Join(dir, fmt.Sprintf("temp_%d_%s.dmg", time.Now().UnixNano(), file))
	if err := os.Rename(config.FileName, tempFileName); err != nil {
		return fmt.Errorf("failed to rename DMG file: %w", err)
	}
	defer os.Remove(tempFileName) // Ensure cleanup of temp file
	if err := hdiutil.Convert(ctx, tempFileName, hdiutil.UDRO, config.FileName); err != nil {
		return fmt.Errorf("failed to convert DMG: %w", err)
	}
	if config.Icon != "" {
		if err := setFileIcon(config.FileName, config.Icon); err != nil {
			return err
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

func tmpMount(ctx context.Context, dmgPath string, process func(dmgFilePath string, mountPoint string) error) error {
	// Create temporary mount point
	tempDir, err := os.MkdirTemp("", "*-zapp-dmg")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)
	mountPoint := filepath.Join(tempDir, "mount")
	if err = hdiutil.Attach(ctx, dmgPath, mountPoint); err != nil {
		return fmt.Errorf("failed to attach DMG: %w", err)
	}
	if err = checkMountVisible(mountPoint); err != nil {
		_ = hdiutil.Detach(ctx, mountPoint)
		return err
	}
	defer func() {
		if err = hdiutil.Detach(ctx, mountPoint); err != nil {
			fmt.Printf("failed to detach DMG: %s", err)
		}
	}()
	return process(dmgPath, mountPoint)
}

// setDMGIcon gives the mounted volume a custom icon. A volume takes its icon
// from a .VolumeIcon.icns file at its root rather than from a resource fork.
func setDMGIcon(mountPoint, iconPath string) error {
	iconFile := filepath.Join(mountPoint, ".VolumeIcon.icns")
	if err := fsutil.CopyFile(iconPath, iconFile); err != nil {
		return fmt.Errorf("failed to copy icon to mount point: %w", err)
	}
	if err := setCreatorCode(iconFile, "icnC"); err != nil {
		return fmt.Errorf("failed to set creator code on volume icon: %w", err)
	}
	if err := markCustomIcon(mountPoint); err != nil {
		return fmt.Errorf("failed to mark volume as having a custom icon: %w", err)
	}
	return nil
}

// setupSourceDirectory sets up the source directory with the necessary files.
func setupSourceDirectory(config Config, sourceDir string) error {
	// Copy the application and other files to the source directory
	for _, item := range config.Contents {
		switch item.Type {

		case File:
			// Copy the file to the source directory
			destPath := filepath.Join(sourceDir, filepath.Base(item.Path))
			if err := fsutil.CopyFile(item.Path, destPath); err != nil {
				return fmt.Errorf("failed to copy file %s to %s: %s", item.Path, destPath, err)
			}
		case Dir:
			// Copy the file to the source directory
			destPath := filepath.Join(sourceDir, filepath.Base(item.Path))
			if err := fsutil.CopyDir(item.Path, destPath); err != nil {
				return fmt.Errorf("failed to copy dir %s to %s: %s", item.Path, destPath, err)
			}
		case Link:
			// Create a symbolic link
			err := os.Symlink(item.Path, filepath.Join(sourceDir, filepath.Base(item.Path)))
			if err != nil {
				return fmt.Errorf("failed to create symbolic link %s: %s", item.Path, err)
			}
		}
	}

	// Copy the background image.
	if config.Background != "" {
		backgroundDir := filepath.Join(sourceDir, ".background")
		if err := os.MkdirAll(backgroundDir, 0755); err != nil {
			return fmt.Errorf("failed to create .background directory: %w", err)
		}
		if err := fsutil.CopyFile(config.Background, filepath.Join(backgroundDir, "background.png")); err != nil {
			return fmt.Errorf("failed to copy background: %s", err)
		}
	}
	return nil
}

// checkMountVisible confirms the attached volume is readable from this process.
// hdiutil reports success once it has attached the image in its own view of the
// filesystem, which is not necessarily this one: under Darling the tool runs
// inside a container, and a mount it makes there may not propagate to the host.
// The steps that follow write the window settings and the background image into
// the volume, so an invisible mount would silently produce a bare disk image.
func checkMountVisible(mountPoint string) error {
	entries, err := os.ReadDir(mountPoint)
	if err != nil {
		return fmt.Errorf("the attached volume is not readable at %s: %w", mountPoint, err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("the volume attached at %s appears empty from this process, "+
			"so its contents cannot be customised", mountPoint)
	}
	return nil
}
