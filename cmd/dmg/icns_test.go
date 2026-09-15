package dmg

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/ironpark/zapp/pkg/icns"
)

func TestEmbeddedDiskIcon(t *testing.T) {
	img, err := readIcnsFromBytes(defaultIconFile)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() < 512 || img.Bounds().Dy() < 512 {
		t.Fatalf("unexpected bounds: %v", img.Bounds())
	}
}

func TestCreateICNS(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 200, G: 100, B: 50, A: 128})
		}
	}
	path := filepath.Join(t.TempDir(), "icon.icns")
	if err := createIcns(src, path); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	family, err := icns.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range []string{"is32", "il32", "ic07", "ic08", "ic09", "ic11", "ic12", "ic13", "ic14"} {
		img, err := family.Image(typ)
		if err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		c := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA)
		if c.A != 128 {
			t.Fatalf("%s: alpha=%d", typ, c.A)
		}
	}
	img, err := readIcns(path)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 512 {
		t.Fatal(img.Bounds())
	}
}
