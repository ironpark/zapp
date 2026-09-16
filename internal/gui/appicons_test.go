package gui

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/ironpark/zapp/pkg/icns"
)

func TestAppIconWithout256PixelRepresentation(t *testing.T) {
	g := testEditor(t)
	app := filepath.Join(t.TempDir(), "Demo.app")
	resources := filepath.Join(app, "Contents", "Resources")
	if err := os.MkdirAll(resources, 0700); err != nil {
		t.Fatal(err)
	}
	data := []byte(`<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleIconFile</key><string>Custom</string></dict></plist>`)
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), data, 0600); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			img.SetRGBA(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	family := &icns.ICNS{}
	if err := family.Add(img); err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err := icns.Encode(&encoded, family); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resources, "Custom.icns"), encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	g.loadAppIcon("test-app", app)
	if g.assets["test-app"] == nil {
		t.Fatal("app icon without 256px variant was ignored")
	}
}
