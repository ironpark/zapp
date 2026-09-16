package dmg

import (
	"bytes"
	"context"
	"encoding/binary"
	"github.com/ironpark/zapp/pkg/icns"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ironpark/zapp/pkg/macfs"
)

func TestItemIconsOnDisk(t *testing.T) {
	for _, filesystem := range []FileSystem{HFSPlus, APFS} {
		t.Run(filesystem.String(), func(t *testing.T) {
			c := appearanceConfig(t, filesystem, 0)
			dir := filepath.Dir(c.FileName)
			file := filepath.Join(dir, "notes.txt")
			if err := os.WriteFile(file, []byte("original contents"), 0644); err != nil {
				t.Fatal(err)
			}
			folder := filepath.Join(dir, "Folder")
			if err := os.Mkdir(folder, 0755); err != nil {
				t.Fatal(err)
			}
			c.Contents = []Item{
				{Type: File, Path: file, Icon: c.Background, X: 100, Y: 100},
				{Type: Dir, Path: folder, Icon: c.Icon, X: 250, Y: 100},
				{Type: Dir, Path: c.Contents[0].Path, Icon: c.Background, X: 400, Y: 100},
			}
			if err := CreateDMG(context.Background(), c); err != nil {
				t.Fatal(err)
			}
			if string(mustRead(t, file)) != "original contents" {
				t.Fatal("source file modified")
			}
			if _, err := os.Stat(filepath.Join(folder, "Icon\r")); !os.IsNotExist(err) {
				t.Fatal("source folder modified")
			}
			if runtime.GOOS != "darwin" {
				return
			}
			point := mount(t, c.FileName)
			for _, item := range c.Contents {
				path := filepath.Join(point, item.ImageName())
				info := readAttr(t, path, finderInfoAttr)
				if binary.BigEndian.Uint16(info[8:])&hasCustomIcon == 0 {
					t.Fatal("custom icon flag missing")
				}
				if item.Type == Dir {
					path = filepath.Join(path, "Icon\r")
				}
				fork := readAttr(t, path, resourceForkAttr)
				expected, err := readItemIcon(item.Icon)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Contains(fork, expected) {
					t.Fatal("mounted icon resource does not match")
				}
			}
		})
	}
}

func TestIconResourcePreservesOtherResources(t *testing.T) {
	original, err := buildResourceFork("TEXT", 42, []byte("keep me"))
	if err != nil {
		t.Fatal(err)
	}
	merged, err := iconResourceFork(macfs.Bytes(original), sampleICNS)
	if err != nil {
		t.Fatal(err)
	}
	merged, err = iconResourceFork(macfs.Bytes(merged), sampleICNS)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(merged, []byte("keep me")) {
		t.Fatal("lost resource data")
	}
	if _, err := iconResourceFork(macfs.Bytes([]byte("malformed")), sampleICNS); err == nil {
		t.Fatal("accepted malformed existing resource fork")
	}
	if runtime.GOOS != "darwin" {
		return
	}
	if _, err := exec.LookPath("DeRez"); err != nil {
		t.Skip("DeRez unavailable")
	}
	path := filepath.Join(t.TempDir(), "resource")
	os.WriteFile(path, []byte("data"), 0644)
	if err := setXattr(path, resourceForkAttr, merged); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command("DeRez", path).CombinedOutput()
	if err != nil || !strings.Contains(string(output), "data 'TEXT' (42") || strings.Count(string(output), "data 'icns' (-16455") != 1 {
		t.Fatalf("invalid merged fork: %v\n%s", err, output)
	}
}

func TestItemIconRestrictions(t *testing.T) {
	c := appearanceConfig(t, HFSPlus, 0)
	c.Contents[1].Icon = c.Background
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "symbolic links") {
		t.Fatalf("link icon accepted: %v", err)
	}
	c.Contents = c.Contents[:1]
	c.Contents[0].Icon = c.Background
	signature := filepath.Join(c.Contents[0].Path, "Contents/_CodeSignature")
	os.MkdirAll(signature, 0755)
	os.WriteFile(filepath.Join(signature, "CodeResources"), []byte("signature"), 0644)
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "signed bundle") {
		t.Fatalf("signed bundle accepted: %v", err)
	}
}

func TestSignedExecutableIconRejected(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("requires a signed Mach-O fixture")
	}
	if !signedMachO("/usr/bin/true") {
		t.Fatal("signed executable was not detected")
	}
	if err := validateItemIconTarget(Item{Type: File, Path: "/usr/bin/true"}); err == nil {
		t.Fatal("signed executable accepted")
	}
}

func TestJPEGItemIcons(t *testing.T) {
	dir := t.TempDir()
	img := image.NewNRGBA(image.Rect(0, 0, 80, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 80; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 220, G: 40, B: 90, A: 255})
		}
	}
	for _, ext := range []string{".jpg", ".jpeg", ".JPG"} {
		path := filepath.Join(dir, "icon"+ext)
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := jpeg.Encode(f, img, nil); err != nil {
			t.Fatal(err)
		}
		f.Close()
		node := &imageNode{Name: "file", Mode: 0644, Data: macfs.Bytes([]byte("data"))}
		if err := applyNodeIcon(node, path); err != nil {
			t.Fatal(err)
		}
		data, err := readItemIcon(path)
		if err != nil {
			t.Fatal(err)
		}
		family, err := icns.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := family.ByResolution(256)
		if err != nil {
			t.Fatal(err)
		}
		_, _, _, alpha := decoded.At(0, 0).RGBA()
		if alpha != 0 {
			t.Fatal("aspect ratio padding lost")
		}
		r, g, _, a := decoded.At(128, 128).RGBA()
		if a != 65535 || r <= g {
			t.Fatal("JPEG color not retained")
		}
	}
}
