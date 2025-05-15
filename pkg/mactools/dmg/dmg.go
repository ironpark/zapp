package dmg

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ironpark/zapp/pkg/mactools/dsstore"
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
func CreateDMG(config Config, sourceDir string) error {
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
	ctx := context.Background()
	// Create the DMG file using hdiutil
	if err := hdiutil.Create(ctx, config.Title, sourceDir, hdiutil.UDRW, config.FileName); err != nil {
		return fmt.Errorf("failed to create dmg: %w", err)
	}

	// Set custom icon for the DMG if specified
	if config.Icon != "" || config.Background != "" {
		err = tmpMount(config.FileName, func(dmgFilePath string, mountPoint string) error {
			if config.Icon != "" {
				if err := setDMGIcon(mountPoint, config.Icon); err != nil {
					return fmt.Errorf("failed to set DMG icon: %w", err)
				}
			}
			if config.Background != "" {
				store.SetBackgroundImage(filepath.Join(mountPoint, ".background", "background.png"))
				if err := store.Write(filepath.Join(mountPoint, ".DS_Store")); err != nil {
					return fmt.Errorf("failed to write .DS_Store: %w", err)
				}
			}
			return nil
		})
	}

	// Convert the DMG to read-only
	tempFileName := "temp_" + config.FileName
	if err := os.Rename(config.FileName, tempFileName); err != nil {
		return fmt.Errorf("failed to rename DMG file: %w", err)
	}
	defer os.Remove(tempFileName) // Ensure cleanup of temp file
	if err := hdiutil.Convert(ctx, tempFileName, hdiutil.UDRO, config.FileName); err != nil {
		return fmt.Errorf("failed to convert DMG: %w", err)
	}
	if config.Icon != "" {
		setFileIcon(config.FileName, config.Icon)
	}
	return nil
}

func setFileIcon(dmgPath, iconPath string) error {
	// Create temporary mount point
	tempDir, err := os.MkdirTemp("", "*-zapp-dmg")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir) // Ensure cleanup of temp directory

	tempIconPath := filepath.Join(tempDir, "icon.icns")
	err = copyFile(iconPath, tempIconPath)
	if err != nil {
		return fmt.Errorf("failed to copy icon: %w", err)
	}

	cmd := exec.Command("sips", "-i", tempIconPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set icon: %s, output: %s", err, string(output))
	}
	cmd = exec.Command("DeRez", "-only", "icns", tempIconPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to DeRez icon: %s, output: %s", err, string(output))
	}
	rsrcPath := filepath.Join(tempDir, "icns.rsrc")
	if err := os.WriteFile(rsrcPath, output, 0644); err != nil {
		return fmt.Errorf("failed to write icns.rsrc: %w", err)
	}
	cmd = exec.Command("Rez", "-append", rsrcPath, "-o", dmgPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to append icns.rsrc: %s, output: %s", err, string(output))
	}
	cmd = exec.Command("SetFile", "-a", "C", dmgPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set icon: %s, output: %s", err, string(output))
	}
	return nil
}

func tmpMount(dmgPath string, process func(dmgFilePath string, mountPoint string) error) error {
	// Create temporary mount point
	tempDir, err := os.MkdirTemp("", "*-zapp-dmg")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)
	mountPoint := filepath.Join(tempDir, "mount")
	ctx := context.Background()
	if err = hdiutil.Attach(ctx, dmgPath, mountPoint); err != nil {
		return fmt.Errorf("failed to attach DMG: %w", err)
	}
	defer func() {
		if err = hdiutil.Detach(ctx, mountPoint); err != nil {
			fmt.Printf("failed to detach DMG: %s", err)
		}
	}()
	return process(dmgPath, mountPoint)
}

func setDMGIcon(mountPoint, iconPath string) error {
	// Copy the icon to the mount point
	iconFile := filepath.Join(mountPoint, ".VolumeIcon.icns")
	if err := copyFile(iconPath, iconFile); err != nil {
		return fmt.Errorf("failed to copy icon to mount point: %w", err)
	}

	// Set the icon
	cmd := exec.Command("SetFile", "-c", "icnC", iconFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set icon: %s, output: %s", err, string(output))
	}

	// Tell the volume that it has a special file attribute
	cmd = exec.Command("SetFile", "-a", "C", mountPoint)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set icon: %s, output: %s", err, string(output))
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

	// 배경 이미지 복사
	if config.Background != "" {
		backgroundDir := filepath.Join(sourceDir, ".background")
		if err := os.MkdirAll(backgroundDir, 0755); err != nil {
			return fmt.Errorf("failed to create .background directory: %w", err)
		}
		if err := copyFile(config.Background, filepath.Join(backgroundDir, "background.png")); err != nil {
			return fmt.Errorf("failed to copy background: %s", err)
		}
	}
	return nil
}

func isDir(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	// handle symbol link
	if fi.Mode()&os.ModeSymlink != 0 {
		realPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			return false, err
		}
		fi, err = os.Stat(realPath)
		if err != nil {
			return false, err
		}
	}

	return fi.IsDir(), nil
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

		entryPath := srcPath
		is_dir, err := isDir(entryPath)
		if err != nil {
			continue
		}

		if is_dir  {
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
