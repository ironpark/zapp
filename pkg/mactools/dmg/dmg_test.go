package dmg

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ironpark/zapp/pkg/mactools/hdiutil"
)

// requireTools skips the test when the macOS tooling CreateDMG shells out to is
// unavailable, so the package still tests cleanly off a Mac.
func requireTools(t *testing.T, names ...string) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping disk image integration test in -short mode")
	}
	for _, name := range names {
		if _, err := exec.LookPath(name); err != nil {
			t.Skipf("%s not available: %v", name, err)
		}
	}
}

// fakeAppBundle builds a minimal .app directory tree. CreateDMG only copies the
// tree and records icon positions by base name, so it needs no real Mach-O.
func fakeAppBundle(t *testing.T, dir, name string) string {
	t.Helper()
	app := filepath.Join(dir, name)
	macOS := filepath.Join(app, "Contents", "MacOS")
	if err := os.MkdirAll(macOS, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(macOS, "App"), []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>App</string>
<key>CFBundleIdentifier</key><string>dev.zapp.test</string>
</dict></plist>`
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte(plist), 0644); err != nil {
		t.Fatal(err)
	}
	return app
}

func fakeBackground(t *testing.T, path string) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	for x := 0; x < 640; x++ {
		for y := 0; y < 480; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}

// mountedNames attaches the image and lists the entries at its root.
func mountedNames(t *testing.T, dmgPath string) map[string]bool {
	t.Helper()
	mountPoint := filepath.Join(t.TempDir(), "mount")
	ctx := context.Background()
	if err := hdiutil.Attach(ctx, dmgPath, mountPoint); err != nil {
		t.Fatalf("failed to attach %s: %v", dmgPath, err)
	}
	t.Cleanup(func() {
		if err := hdiutil.Detach(ctx, mountPoint); err != nil {
			t.Errorf("failed to detach %s: %v", mountPoint, err)
		}
	})
	entries, err := os.ReadDir(mountPoint)
	if err != nil {
		t.Fatalf("failed to read mount point: %v", err)
	}
	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name()] = true
	}
	return names
}

// chdir switches to dir for the duration of the test. (testing.T.Chdir needs
// go1.24; this module targets go1.22.)
func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(prev) })
}

func TestCreateDMG(t *testing.T) {
	requireTools(t, "hdiutil")

	fixtures := t.TempDir()
	app := fakeAppBundle(t, fixtures, "SyncMaster.app")
	out := filepath.Join(t.TempDir(), "SyncMaster.dmg")

	var log bytes.Buffer
	err := CreateDMG(context.Background(), Config{
		FileName:         out,
		Title:            "SyncMaster",
		LabelSize:        15,
		ContentsIconSize: 200,
		WindowWidth:      640,
		WindowHeight:     480,
		LogWriter:        &log,
		Contents: []Item{
			{X: 128, Y: 240, Type: Dir, Path: app},
			{X: 512, Y: 240, Type: Link, Path: "/Applications"},
		},
	}, filepath.Join(t.TempDir(), "src"))
	if err != nil {
		t.Fatalf("CreateDMG() error: %v", err)
	}

	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("output DMG not created: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("output DMG is empty")
	}

	names := mountedNames(t, out)
	if !names["SyncMaster.app"] {
		t.Errorf("app bundle missing from image; got entries %v", names)
	}
	if !names["Applications"] {
		t.Errorf("/Applications symlink missing from image; got entries %v", names)
	}

	// The temp image CreateDMG converts from must not survive next to the
	// output.
	siblings, err := os.ReadDir(filepath.Dir(out))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range siblings {
		if s.Name() != filepath.Base(out) {
			t.Errorf("leftover file next to output DMG: %s", s.Name())
		}
	}
}

func TestCreateDMGWithBackground(t *testing.T) {
	requireTools(t, "hdiutil")

	fixtures := t.TempDir()
	app := fakeAppBundle(t, fixtures, "SyncMaster.app")
	bg := fakeBackground(t, filepath.Join(fixtures, "bg.png"))
	out := filepath.Join(t.TempDir(), "SyncMaster.dmg")

	err := CreateDMG(context.Background(), Config{
		FileName:         out,
		Title:            "SyncMaster",
		LabelSize:        15,
		ContentsIconSize: 200,
		WindowWidth:      640,
		WindowHeight:     480,
		Background:       bg,
		LogWriter:        &bytes.Buffer{},
		Contents: []Item{
			{X: 128, Y: 240, Type: Dir, Path: app},
			{X: 512, Y: 240, Type: Link, Path: "/Applications"},
		},
	}, filepath.Join(t.TempDir(), "src"))
	if err != nil {
		t.Fatalf("CreateDMG() error: %v", err)
	}

	names := mountedNames(t, out)
	if !names[".background"] {
		t.Errorf("background directory missing from image; got entries %v", names)
	}
	if !names[".DS_Store"] {
		t.Errorf(".DS_Store missing from image; got entries %v", names)
	}
}

// CreateDMG defaults the output name from the title when FileName is empty.
func TestCreateDMGDefaultFileName(t *testing.T) {
	requireTools(t, "hdiutil")

	work := t.TempDir()
	app := fakeAppBundle(t, work, "SyncMaster.app")

	// FileName is resolved relative to the working directory.
	chdir(t, work)

	err := CreateDMG(context.Background(), Config{
		Title:            "SyncMaster",
		LabelSize:        15,
		ContentsIconSize: 128,
		WindowWidth:      640,
		WindowHeight:     480,
		LogWriter:        &bytes.Buffer{},
		Contents:         []Item{{X: 128, Y: 240, Type: Dir, Path: app}},
	}, filepath.Join(t.TempDir(), "src"))
	if err != nil {
		t.Fatalf("CreateDMG() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(work, "SyncMaster.dmg")); err != nil {
		t.Errorf("expected SyncMaster.dmg to be created: %v", err)
	}
}
