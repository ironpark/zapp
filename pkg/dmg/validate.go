package dmg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ironpark/zapp/internal/thirdparty/text/unicode/norm"
)

// ImageName is the item's name inside the volume and in Finder layout records.
func (i Item) ImageName() string {
	if i.Name != "" {
		return i.Name
	}
	return filepath.Base(i.Path)
}

// Layout bounds accepted by Validate. They are exported so editors can reject
// out-of-range input with the same limits image generation enforces.
const (
	MinIconSize  = 16
	MaxIconSize  = 512
	MinLabelSize = 10
	MaxLabelSize = 16
	// MaxCoordinate is the largest icon position a Finder layout record holds.
	MaxCoordinate = 0xffffffff
)

// Validate checks layout and source inputs before image generation.
func (c Config) Validate() error {
	if c.Title == "" {
		return fmt.Errorf("a volume title is required")
	}
	if c.WindowWidth <= 0 || c.WindowHeight <= 0 {
		return fmt.Errorf("window dimensions must be positive")
	}
	if c.ContentsIconSize < MinIconSize || c.ContentsIconSize > MaxIconSize {
		return fmt.Errorf("icon size must be between %d and %d", MinIconSize, MaxIconSize)
	}
	if c.LabelSize < MinLabelSize || c.LabelSize > MaxLabelSize {
		return fmt.Errorf("label size must be between %d and %d", MinLabelSize, MaxLabelSize)
	}
	if len(c.Contents) == 0 {
		return fmt.Errorf("contents must not be empty")
	}
	seen := map[string]bool{}
	for _, name := range []string{storeName, backgroundDir, volumeIconName} {
		seen[strings.ToLower(name)] = true
	}
	for _, item := range c.Contents {
		name := item.ImageName()
		if item.Path == "" || name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\:\x00") {
			return fmt.Errorf("invalid content name or path %q", name)
		}
		key := strings.ToLower(norm.NFD.String(name))
		if seen[key] {
			return fmt.Errorf("duplicate or reserved content name %q", name)
		}
		seen[key] = true
		if item.X < 0 || item.Y < 0 || uint64(item.X) > MaxCoordinate || uint64(item.Y) > MaxCoordinate {
			return fmt.Errorf("invalid coordinates for %q", name)
		}
		switch item.Type {
		case Link:
			if strings.ContainsRune(item.Path, 0) {
				return fmt.Errorf("invalid link target for %q", name)
			}
		case File, Dir:
			info, err := os.Lstat(item.Path)
			if err != nil {
				return fmt.Errorf("content %q: %w", name, err)
			}
			if item.Type == Dir && !info.IsDir() {
				return fmt.Errorf("%s is not a directory", item.Path)
			}
			if item.Type == File && !info.Mode().IsRegular() {
				return fmt.Errorf("%s is not a regular file", item.Path)
			}
		default:
			return fmt.Errorf("unknown content type %q", item.Type)
		}
	}
	return nil
}
