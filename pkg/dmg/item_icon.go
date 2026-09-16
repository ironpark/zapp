package dmg

import (
	"bytes"
	"debug/macho"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ironpark/zapp/internal/imageutil"
	"github.com/ironpark/zapp/pkg/icns"
	"github.com/ironpark/zapp/pkg/macfs"
)

// readItemIcon normalizes PNG/JPEG images into an icon family; ICNS families retain all sizes.
func readItemIcon(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var family *icns.ICNS
	switch strings.ToLower(filepath.Ext(path)) {
	case ".icns":
		family, err = icns.Decode(f)
		if err == nil {
			_, err = family.HighestResolution()
		}
	case ".png", ".jpg", ".jpeg":
		var config image.Config
		config, _, err = image.DecodeConfig(f)
		if err != nil {
			return nil, err
		}
		if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 32<<20 {
			return nil, fmt.Errorf("icon image is too large")
		}
		if _, err = f.Seek(0, 0); err != nil {
			return nil, err
		}
		var img image.Image
		img, _, err = image.Decode(f)
		if err != nil {
			return nil, err
		}
		family = icns.NewICNS()
		for _, size := range []int{16, 32, 64, 128, 256, 512} {
			w, h := img.Bounds().Dx(), img.Bounds().Dy()
			longest := max(w, h)
			scaled := imageutil.Resize(img, max(1, w*size/longest), max(1, h*size/longest))
			square := image.NewNRGBA(image.Rect(0, 0, size, size))
			offset := image.Pt((size-scaled.Bounds().Dx())/2, (size-scaled.Bounds().Dy())/2)
			draw.Draw(square, scaled.Bounds().Sub(scaled.Bounds().Min).Add(offset), scaled, scaled.Bounds().Min, draw.Src)
			if err = family.Add(square); err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("item icon must be PNG, JPEG or ICNS: %s", path)
	}
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err := icns.Encode(&output, family); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func applyNodeIcon(node *imageNode, path string) error {
	data, err := readItemIcon(path)
	if err != nil {
		return err
	}
	carrier := node
	if node.IsDir() {
		carrier = node.Child("Icon\r")
		if carrier == nil {
			carrier = &imageNode{Name: "Icon\r", Mode: fs.FileMode(0644), ModTime: node.ModTime, Data: macfs.Bytes(nil)}
			node.Children = append(node.Children, carrier)
		}
		if carrier.IsDir() || carrier.IsSymlink() {
			return fmt.Errorf("existing Icon file is not a regular file")
		}
	}
	fork, err := iconResourceFork(carrier.ResourceFork, data)
	if err != nil {
		return err
	}
	carrier.ResourceFork = macfs.Bytes(fork)
	flags := binary.BigEndian.Uint16(node.FinderInfo[8:])
	binary.BigEndian.PutUint16(node.FinderInfo[8:], flags|hasCustomIcon)
	if carrier != node {
		binary.BigEndian.PutUint16(carrier.FinderInfo[8:], binary.BigEndian.Uint16(carrier.FinderInfo[8:])|0x4000)
	}
	return nil
}

// Signed bundles reject Finder metadata; do not silently invalidate a signature.
func validateItemIconTarget(item Item) error {
	if item.Type == Link {
		return fmt.Errorf("custom icons are not supported for symbolic links: %s", item.Path)
	}
	for _, relative := range []string{"Contents/_CodeSignature/CodeResources", "_CodeSignature/CodeResources"} {
		_, err := os.Stat(filepath.Join(item.Path, relative))
		if err == nil {
			return fmt.Errorf("custom icon would invalidate signed bundle %q; change its app icon before signing", item.Path)
		}
	}
	if signedMachO(item.Path) {
		return fmt.Errorf("custom icon would invalidate signed executable %q", item.Path)
	}
	return nil
}

func signedMachO(path string) bool {
	hasSignature := func(file *macho.File) bool {
		for _, load := range file.Loads {
			raw := load.Raw()
			if len(raw) >= 4 && file.ByteOrder.Uint32(raw) == 0x1d {
				return true
			} // LC_CODE_SIGNATURE
		}
		return false
	}
	if fat, err := macho.OpenFat(path); err == nil {
		defer fat.Close()
		for _, arch := range fat.Arches {
			if hasSignature(arch.File) {
				return true
			}
		}
		return false
	}
	if file, err := macho.Open(path); err == nil {
		defer file.Close()
		return hasSignature(file)
	}
	return false
}
