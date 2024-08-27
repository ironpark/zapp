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

	"github.com/nfnt/resize"
	"howett.net/plist"
	"yrh.dev/icns"
)

func getAppIconPath(appPath string) (string, error) {
	if !strings.HasSuffix(appPath, ".app") {
		return "", fmt.Errorf("not an app: %s", appPath)
	}
	plistPath := filepath.Join(appPath, "Contents", "Info.plist")
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return "", fmt.Errorf("failed to read plist file: %v", err)
	}

	var plistData map[string]interface{}
	_, err = plist.Unmarshal(data, &plistData)
	if err != nil {
		return "", fmt.Errorf("failed to parse plist: %v", err)
	}
	if plistData["CFBundleIconFile"] == nil {
		return "", fmt.Errorf("icon file not found in plist")
	}
	appIcon := filepath.Join(appPath, "Contents", "Resources", plistData["CFBundleIconFile"].(string))
	if filepath.Ext(appIcon) == "" {
		appIcon += ".icns"
	}
	return appIcon, nil
}

func createIconSet(iconPath string, output string, withDiskBg bool) error {
	var iconImage image.Image
	var err error
	// is app bundle
	switch filepath.Ext(iconPath) {
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
	diskImage = resize.Resize(512, 512, diskImage, resize.Lanczos3)
	iconImage = resize.Resize(256, 256, iconImage, resize.Lanczos3)
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
		return nil, fmt.Errorf("failed to open disk image: %w", err)
	}
	defer diskImg.Close()
	// Decode the disk image
	img, err := png.Decode(diskImg)
	if err != nil {
		return nil, fmt.Errorf("failed to decode disk image: %w", err)
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

	defer icnsFile.Close()
	icnsImg := icns.NewICNS(icns.WithMinCompatibility(icns.MountainLion))
	for _, size := range []int{32, 64, 128, 256, 512} {
		resizedImg := resize.Resize(uint(size), uint(size), img, resize.Lanczos3)
		icnsImg.Add(resizedImg)
	}
	// Encode the image as ICNS
	if err := icns.Encode(icnsFile, icnsImg); err != nil {
		return fmt.Errorf("failed to encode ICNS: %w", err)
	}
	return nil
}

func createIcnsFromPng(imgPath string, icnsPath string) error {
	// Read the PNG file
	img, err := readPng(imgPath)
	if err != nil {
		return err
	}
	return createIcns(img, icnsPath)
}
