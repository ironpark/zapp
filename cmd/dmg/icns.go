package dmg

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/ironpark/zapp/internal/imageutil"
	"github.com/ironpark/zapp/pkg/appbundle"
	"github.com/ironpark/zapp/pkg/icns"
)

func getAppIconPath(appPath string) (string, error) {
	if !strings.HasSuffix(appPath, ".app") {
		return "", fmt.Errorf("not an app: %s", appPath)
	}
	info, err := appbundle.Open(appPath)
	if err != nil {
		return "", err
	}
	return info.IconFilePath()
}

func createIconSet(iconPath string, output string, withDiskBg bool) error {
	var iconImage image.Image
	var err error
	// is app bundle
	switch strings.ToLower(filepath.Ext(iconPath)) {
	case ".icns":
		iconImage, err = readIcns(iconPath)
		if err != nil {
			return err
		}
	case ".app":
		iconPath, err := getAppIconPath(iconPath)
		if err != nil {
			return err
		}
		iconImage, err = readIcns(iconPath)
		if err != nil {
			return err
		}
	case ".png":
		iconImage, err = readPng(iconPath)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported icon file: %s", iconPath)
	}
	if withDiskBg {
		diskImg, err := readIcnsFromBytes(defaultIconFile)
		if err != nil {
			return err
		}
		return createIcns(mixDraw(diskImg, iconImage), output)
	} else {
		return createIcns(iconImage, output)
	}
}

func mixDraw(diskImage image.Image, iconImage image.Image) draw.Image {
	diskImage = imageutil.Resize(diskImage, 512, 512)
	iconImage = fitIcon(iconImage, 256)
	// Create result image (same size as disk image)
	result := image.NewRGBA(diskImage.Bounds())

	// Draw disk image onto result image
	draw.Draw(result, result.Bounds(), diskImage, image.Point{}, draw.Src)

	// Calculate center position for icon image
	posX := (diskImage.Bounds().Dx() - iconImage.Bounds().Dx()) / 2
	posY := (diskImage.Bounds().Dy() - iconImage.Bounds().Dy()) / 3

	// Draw icon image at the center of the result image
	draw.Draw(result, image.Rect(posX, posY, posX+iconImage.Bounds().Dx(), posY+iconImage.Bounds().Dy()), iconImage, image.Point{}, draw.Over)

	return result
}

func readPng(filename string) (image.Image, error) {
	// Read the disk image
	diskImg, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open PNG icon: %w", err)
	}
	defer func() { _ = diskImg.Close() }()
	// Decode the disk image
	img, err := png.Decode(diskImg)
	if err != nil {
		return nil, fmt.Errorf("failed to decode PNG icon: %w", err)
	}
	return img, nil
}

func readIcnsFromBytes(data []byte) (image.Image, error) {
	// Parse the ICNS data
	icons, err := icns.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode ICNS data: %w", err)
	}

	// Find the largest icon
	img, err := icons.HighestResolution()
	if err != nil {
		return nil, fmt.Errorf("failed to get highest resolution icon: %w", err)
	}
	return img, nil
}

// readIcns extracts the icon image from the icns file
// extracted icon is largest size icon in the icns file
func readIcns(filename string) (image.Image, error) {
	// Read the ICNS file
	icnsData, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read ICNS file: %w", err)
	}
	return readIcnsFromBytes(icnsData)
}

func createIcns(img image.Image, icnsPath string) error {
	// Create a new ICNS file
	icnsFile, err := os.Create(icnsPath)
	if err != nil {
		return fmt.Errorf("failed to create ICNS file: %w", err)
	}

	defer func() { _ = icnsFile.Close() }()
	icnsImg := icns.NewICNS()
	for _, slot := range []struct {
		typ  string
		size int
	}{
		{"is32", 16}, {"il32", 32}, {"ic07", 128}, {"ic08", 256}, {"ic09", 512},
		{"ic11", 32}, {"ic12", 64}, {"ic13", 256}, {"ic14", 512},
	} {
		resizedImg := fitIcon(img, slot.size)
		if err := icnsImg.AddWithType(slot.typ, resizedImg); err != nil {
			return fmt.Errorf("failed to add %s icon: %w", slot.typ, err)
		}
	}
	// Encode the image as ICNS
	if err := icns.Encode(icnsFile, icnsImg); err != nil {
		return fmt.Errorf("failed to encode ICNS: %w", err)
	}
	return icnsFile.Close()
}

// fitIcon preserves the artwork's aspect ratio and centers it on transparent padding.
func fitIcon(img image.Image, size int) *image.RGBA {
	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	if width == height {
		return imageutil.Resize(img, size, size)
	}
	if width > height {
		height = max(1, int(float64(height)*float64(size)/float64(width)))
		width = size
	} else {
		width = max(1, int(float64(width)*float64(size)/float64(height)))
		height = size
	}
	scaled := imageutil.Resize(img, width, height)
	result := image.NewRGBA(image.Rect(0, 0, size, size))
	offset := image.Pt((size-width)/2, (size-height)/2)
	draw.Draw(result, scaled.Bounds().Add(offset), scaled, scaled.Bounds().Min, draw.Src)
	return result
}
