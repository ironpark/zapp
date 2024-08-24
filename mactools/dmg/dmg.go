package dmg

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"zapp/mactools/dsstore"
)

// Config represents the configuration for the DMG file.
type Config struct {
	Title            string `json:"title"`
	Icon             string `json:"icon"`
	LabelSize        int    `json:"labelSize"`
	ContentsIconSize int    `json:"iconSize"`
	WindowWidth      int    `json:"windowWidth"`
	WindowHeight     int    `json:"windowHeight"`
	Background       string `json:"background"`
	Contents         []Item `json:"contents"`
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
func CreateDMG(config Config, sourceDir string) error {
	// Create the source directory if it doesn't exist
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		return fmt.Errorf("failed to create source directory: %w", err)
	}
	// Setup the source directory with the necessary files
	if err := setupSourceDirectory(config, sourceDir); err != nil {
		return fmt.Errorf("failed to setup source directory: %w", err)
	}

	store := dsstore.NewDSStore()
	store.SetIconSize(float64(config.ContentsIconSize))
	store.SetWindow(config.WindowWidth, config.WindowHeight, 0, 0)
	store.SetLabelSize(float64(config.LabelSize))
	store.SetLabelPlaceToBottom(true)

	for _, content := range config.Contents {
		store.SetIconPos(filepath.Base(content.Path), uint32(content.X), uint32(content.Y))
	}
	err := store.Write(filepath.Join(sourceDir, ".DS_Store"))
	if err != nil {
		return fmt.Errorf("failed to write .DS_Store: %w", err)
	}
	// Create the DMG file using hdiutil
	cmd := exec.Command("hdiutil", "create", "-volname", config.Title, "-srcfolder", sourceDir, "-ov", "-format", "UDZO", config.Title+".dmg")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create dmg: %s, output: %s", err, string(output))
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
			if err := copyFile(item.Path, destPath); err != nil {
				return fmt.Errorf("failed to copy file %s to %s: %s", item.Path, destPath, err)
			}
		case Dir:
			// Copy the file to the source directory
			destPath := filepath.Join(sourceDir, filepath.Base(item.Path))
			if err := copyDir(item.Path, destPath); err != nil {
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
	// Copy the icon and background if specified
	if config.Icon != "" {
		if err := copyFile(config.Icon, filepath.Join(sourceDir, ".VolumeIcon.icns")); err != nil {
			return fmt.Errorf("failed to copy icon: %s", err)
		}
	}

	if config.Background != "" {
		if err := copyFile(config.Background, filepath.Join(sourceDir, ".background", "background.png")); err != nil {
			return fmt.Errorf("failed to copy background: %s", err)
		}
	}
	return nil
}

// copyDir copies a directory from src to dst recursively.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err = copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err = copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	return err
}
