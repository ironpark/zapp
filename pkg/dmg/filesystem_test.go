package dmg

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/ironpark/zapp/pkg/plist"
	"github.com/ironpark/zapp/pkg/udif"
)

func appearanceConfig(t *testing.T, filesystem FileSystem, format udif.Format) Config {
	t.Helper()
	dir := t.TempDir()
	bg := filepath.Join(dir, "background.png")
	picture := image.NewRGBA(image.Rect(0, 0, 640, 480))
	for y := range 480 {
		for x := range 640 {
			picture.SetRGBA(x, y, color.RGBA{40, 70, 110, 255})
		}
	}
	f, err := os.Create(bg)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, picture); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	config := Config{FileName: filepath.Join(dir, "image.dmg"), Title: fmt.Sprintf("Zapp %s %08x", filesystem, crc32.ChecksumIEEE([]byte(dir))), FileSystem: filesystem, Format: format,
		Background: bg, Icon: filepath.Join("..", "..", "cmd", "dmg", "iconfile.icns"), Created: time.Unix(1700000000, 0), LabelSize: 14, ContentsIconSize: 128, WindowWidth: 640, WindowHeight: 480,
		Contents: []Item{{X: 100, Y: 200, Type: Dir, Path: sampleApp(t, dir)}, {X: 400, Y: 200, Type: Link, Path: "/Applications"}}}
	executable := filepath.Join(config.Contents[0].Path, "Contents/MacOS/Demo")
	for name, value := range map[string][]byte{finderInfoAttr: importedFinderInfo[:], resourceForkAttr: importedResourceFork} {
		if err := setXattr(executable, name, value); err != nil {
			t.Fatal(err)
		}
	}
	return config
}

var importedFinderInfo = [32]byte{'T', 'E', 'X', 'T', 'Z', 'A', 'P', 'P'}
var importedResourceFork = bytes.Repeat([]byte("resource fork contents"), 100)

func TestFileSystemCompressionMatrix(t *testing.T) {
	var resolver string
	if runtime.GOOS == "darwin" {
		resolver = filepath.Join(t.TempDir(), "resolve-alias")
		if out, err := exec.Command("clang", "-Wno-deprecated-declarations", "-framework", "CoreServices", "testdata/resolve_alias.c", "-o", resolver).CombinedOutput(); err != nil {
			t.Fatalf("compile alias oracle: %v\n%s", err, out)
		}
	}
	for _, filesystem := range []FileSystem{HFSPlus, APFS, APFSCaseSensitive} {
		for _, format := range []udif.Format{udif.UDZO, udif.ULFO} {
			t.Run(filesystem.String()+"/"+format.String(), func(t *testing.T) {
				config := appearanceConfig(t, filesystem, format)
				if format == udif.ULFO {
					config.Created = time.Time{} // Exercise fractional default timestamps.
				}

				if err := CreateDMG(context.Background(), config); err != nil {
					t.Fatal(err)
				}
				b, err := os.ReadFile(config.FileName)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Contains(b, []byte(config.FileSystem.diskType())) {
					t.Fatal("UDIF disk type is missing")
				}
				if !config.Created.IsZero() {
					if err := CreateDMG(context.Background(), config); err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(b, mustRead(t, config.FileName)) {
						t.Fatal("fixed inputs produced different DMG bytes")
					}
				}
				if runtime.GOOS != "darwin" {
					return
				}
				if out, err := exec.Command("hdiutil", "verify", config.FileName).CombinedOutput(); err != nil {
					t.Fatalf("verify: %v\n%s", err, out)
				}
				point := mount(t, config.FileName)
				if got, err := os.ReadFile(filepath.Join(point, "Demo.app/Contents/MacOS/Demo")); err != nil || string(got) != "#!/bin/sh\n" {
					t.Fatalf("app contents: %q %v", got, err)
				}
				if info, err := os.Stat(filepath.Join(point, "Demo.app/Contents/MacOS/Demo")); err != nil || info.Mode().Perm() != 0755 {
					t.Fatalf("executable permission: %v %v", info, err)
				}
				for path, want := range map[string]string{"Applications": "/Applications", "Demo.app/Contents/Frameworks/X.framework/X": "Versions/Current/X", "Demo.app/Contents/Frameworks/X.framework/Versions/Current": "A"} {
					if got, err := os.Readlink(filepath.Join(point, path)); err != nil || got != want {
						t.Fatalf("link %s: %q %v", path, got, err)
					}
				}
				if out, err := exec.Command("diskutil", "info", "-plist", point).Output(); err != nil {
					t.Fatal(err)
				} else {
					info, err := plist.ParseDict(out)
					if err != nil {
						t.Fatal(err)
					}
					want := "hfs"
					if filesystem != HFSPlus {
						want = "apfs"
					}
					if info["FilesystemType"] != want {
						t.Fatalf("filesystem: %v", info["FilesystemType"])
					}
					if filesystem != HFSPlus {
						dev, ok := info["ParentWholeDisk"].(string)
						if !ok {
							t.Fatal("missing APFS container device")
						}
						checkAPFSDevice(t, "/dev/r"+dev)
					}
				}
				assertAppearance(t, point, resolver)
			})
		}
	}
}

// CI downloads the artifacts produced on Linux and validates their Finder
// metadata through the same native oracle used for locally generated images.
func TestImportedArtifacts(t *testing.T) {
	dir := os.Getenv("ZAPP_DMG_IMPORT_DIR")
	if dir == "" {
		t.Skip("set ZAPP_DMG_IMPORT_DIR to check exported images")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("Apple verification needs macOS")
	}
	resolver := filepath.Join(t.TempDir(), "resolve-alias")
	if out, err := exec.Command("clang", "-Wno-deprecated-declarations", "-framework", "CoreServices", "testdata/resolve_alias.c", "-o", resolver).CombinedOutput(); err != nil {
		t.Fatalf("alias oracle: %v\n%s", err, out)
	}
	for _, filesystem := range []FileSystem{HFSPlus, APFS, APFSCaseSensitive} {
		for _, format := range []udif.Format{udif.UDZO, udif.ULFO} {
			t.Run(filesystem.String()+"/"+format.String(), func(t *testing.T) {
				path := filepath.Join(dir, "cross-platform-"+filesystem.String()+"-"+strings.ToLower(format.String())+".dmg")
				if out, err := exec.Command("hdiutil", "verify", path).CombinedOutput(); err != nil {
					t.Fatalf("verify: %v\n%s", err, out)
				}
				out, err := exec.Command("hdiutil", "imageinfo", path).CombinedOutput()
				if err != nil || !strings.Contains(string(out), "Format: "+format.String()) {
					t.Fatalf("format: %v\n%s", err, out)
				}
				point := mount(t, path)
				if got := string(mustRead(t, filepath.Join(point, "Demo.app/Contents/MacOS/Demo"))); got != "#!/bin/sh\n" {
					t.Fatalf("app content: %q", got)
				}
				for name, target := range map[string]string{"Applications": "/Applications", "Demo.app/Contents/Frameworks/X.framework/X": "Versions/Current/X", "Demo.app/Contents/Frameworks/X.framework/Versions/Current": "A"} {
					if got, err := os.Readlink(filepath.Join(point, name)); err != nil || got != target {
						t.Fatalf("link: %q %v", got, err)
					}
				}
				if filesystem != HFSPlus {
					out, err := exec.Command("diskutil", "info", "-plist", point).Output()
					if err != nil {
						t.Fatal(err)
					}
					info, err := plist.ParseDict(out)
					if err != nil {
						t.Fatal(err)
					}
					device, ok := info["ParentWholeDisk"].(string)
					if !ok {
						t.Fatal("missing container")
					}
					checkAPFSDevice(t, "/dev/r"+device)
				}
				assertAppearance(t, point, resolver)
			})
		}
	}
}

func checkAPFSDevice(t *testing.T, device string) {
	t.Helper()
	out, err := exec.Command("/sbin/fsck_apfs", "-n", device).CombinedOutput()
	if err != nil || bytes.Contains(out, []byte("error:")) || bytes.Contains(out, []byte("Fix ")) {
		t.Fatalf("fsck_apfs: %v\n%s", err, out)
	}
}

func assertAppearance(t *testing.T, point, resolver string) {
	t.Helper()
	executable := filepath.Join(point, "Demo.app/Contents/MacOS/Demo")
	var imported [32]byte
	if n, err := getXattr(executable, finderInfoAttr, imported[:]); err != nil || n != 32 || imported != importedFinderInfo {
		t.Fatalf("imported FinderInfo: %x %v", imported, err)
	}
	if got := mustRead(t, filepath.Join(executable, "..namedfork/rsrc")); !bytes.Equal(got, importedResourceFork) {
		t.Fatal("imported resource fork differs")
	}
	b, err := os.ReadFile(filepath.Join(point, ".DS_Store"))
	if err != nil {
		t.Fatal(err)
	}
	for name, x := range map[string]uint32{"Demo.app": 100, "Applications": 400} {
		key := binary.BigEndian.AppendUint32(nil, uint32(len(utf16.Encode([]rune(name)))))
		for _, r := range utf16.Encode([]rune(name)) {
			key = binary.BigEndian.AppendUint16(key, r)
		}
		key = append(key, "Ilocblob"...)
		offset := bytes.Index(b, key)
		if offset < 0 || offset+len(key)+12 > len(b) {
			t.Fatalf("missing icon position for %s", name)
		}
		position := b[offset+len(key):]
		if binary.BigEndian.Uint32(position[4:]) != x || binary.BigEndian.Uint32(position[8:]) != 200 {
			t.Fatalf("incorrect icon position for %s", name)
		}
	}
	start := bytes.Index(b, []byte("icvpblob"))
	if start < 0 || start+12 > len(b) {
		t.Fatal("missing icon preferences")
	}
	size := int(binary.BigEndian.Uint32(b[start+8:]))
	start += 12
	if size > len(b)-start {
		t.Fatal("invalid icon preferences length")
	}
	preferences, err := plist.ParseDict(b[start : start+size])
	if err != nil {
		t.Fatal(err)
	}
	record, ok := preferences["backgroundImageAlias"].([]byte)
	if !ok {
		t.Fatal("missing background alias")
	}
	aliasFile := filepath.Join(t.TempDir(), "background.alias")
	if err := os.WriteFile(aliasFile, record, 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(resolver, aliasFile).CombinedOutput()
	if err != nil {
		t.Fatalf("resolve background alias: %v\n%s", err, out)
	}
	resolved := strings.TrimSpace(string(out))
	want := filepath.Join(point, ".background/background.png")
	gotInfo, err := os.Stat(resolved)
	if err != nil {
		t.Fatal(err)
	}
	wantInfo, err := os.Stat(want)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(gotInfo, wantInfo) {
		t.Fatalf("alias resolved to %q instead of %q", resolved, want)
	}
	var finder [32]byte
	if n, err := getXattr(point, "com.apple.FinderInfo", finder[:]); err != nil || n != 32 || binary.BigEndian.Uint16(finder[8:])&0x400 == 0 {
		t.Fatalf("custom volume icon flag: %x %v", finder, err)
	}
	if icon, err := os.ReadFile(filepath.Join(point, ".VolumeIcon.icns")); err != nil || !bytes.HasPrefix(icon, []byte("icns")) {
		t.Fatalf("volume icon: %v", err)
	}
	if _, err := png.DecodeConfig(bytes.NewReader(mustRead(t, want))); err != nil {
		t.Fatalf("background: %v", err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type failingImage struct{ size int }

func (failingImage) Size() int64 { return 4096 }
func (f failingImage) WriteTo(ctx context.Context, w io.Writer) (int64, error) {
	n, _ := w.Write(make([]byte, f.size))
	return int64(n), fmt.Errorf("source failed")
}

func TestFailedBuildPreservesOutput(t *testing.T) {
	for _, filesystem := range []FileSystem{HFSPlus, APFS, APFSCaseSensitive} {
		t.Run(filesystem.String(), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "existing.dmg")
			original := []byte("existing output")
			if err := os.WriteFile(path, original, 0644); err != nil {
				t.Fatal(err)
			}
			for _, size := range []int{512, 4096} {
				for _, format := range []udif.Format{udif.UDZO, udif.ULFO} {
					c := Config{FileName: path, Format: format, FileSystem: filesystem}
					if err := c.writeImage(context.Background(), failingImage{size: size}); err == nil {
						t.Fatal("expected source failure")
					}
					if !bytes.Equal(mustRead(t, path), original) {
						t.Fatal("failed build replaced output")
					}
				}
			}
			if !bytes.Equal(mustRead(t, path), original) {
				t.Fatal("failed build replaced output")
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := CreateDMG(ctx, Config{Title: "Cancelled", FileName: path, FileSystem: filesystem}); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancel: %v", err)
			}
			if !bytes.Equal(mustRead(t, path), original) {
				t.Fatal("cancelled build replaced output")
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatalf("temporary files leaked: %v %v", entries, err)
			}
		})
	}
}
