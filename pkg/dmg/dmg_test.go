package dmg

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

// sampleApp builds a bundle shaped like a real one, including the web of
// symbolic links a framework inside an application is made of.
func sampleApp(t *testing.T, dir string) string {
	t.Helper()
	app := filepath.Join(dir, "Demo.app")
	versions := filepath.Join(app, "Contents", "Frameworks", "X.framework", "Versions")
	if err := os.MkdirAll(filepath.Join(versions, "A"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0755); err != nil {
		t.Fatal(err)
	}
	write := func(path, contents string, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(path, []byte(contents), mode); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(app, "Contents", "Info.plist"), "<plist/>", 0644)
	write(filepath.Join(app, "Contents", "MacOS", "Demo"), "#!/bin/sh\n", 0755)
	write(filepath.Join(versions, "A", "X"), "framework\n", 0644)
	if err := os.Symlink("A", filepath.Join(versions, "Current")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("Versions/Current/X", filepath.Join(app, "Contents", "Frameworks", "X.framework", "X")); err != nil {
		t.Fatal(err)
	}
	return app
}

// mount attaches an image and returns where it is readable. Reading an image
// back is only possible on macOS.
func mount(t *testing.T, image string) string {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("reading an image back needs macOS")
	}
	point := filepath.Join(t.TempDir(), "mnt")
	if err := os.MkdirAll(point, 0755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("hdiutil", "attach", image, "-mountpoint", point, "-nobrowse", "-readonly").CombinedOutput(); err != nil {
		t.Fatalf("attach: %v\n%s", err, out)
	}
	t.Cleanup(func() { _ = exec.Command("hdiutil", "detach", point, "-force").Run() })
	return point
}

func TestCreateDMG(t *testing.T) {
	dir := t.TempDir()
	app := sampleApp(t, dir)
	background := filepath.Join(dir, "bg.png")
	if err := os.WriteFile(background, []byte("not really a png, but bytes all the same"), 0644); err != nil {
		t.Fatal(err)
	}
	icon := filepath.Join(dir, "icon.icns")
	if err := os.WriteFile(icon, []byte("icns"), 0644); err != nil {
		t.Fatal(err)
	}

	output := filepath.Join(dir, "Demo.dmg")
	err := CreateDMG(context.Background(), Config{
		FileName: output, Title: "Demo", Icon: icon, Background: background,
		LabelSize: 14, ContentsIconSize: 128, WindowWidth: 640, WindowHeight: 480,
		Created: time.Unix(1700000000, 0),
		Contents: []Item{
			{X: 100, Y: 200, Type: Dir, Path: app},
			{X: 400, Y: 200, Type: Link, Path: "/Applications"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if runtime.GOOS == "darwin" {
		if out, err := exec.Command("hdiutil", "verify", output).CombinedOutput(); err != nil {
			t.Fatalf("the image did not verify: %v\n%s", err, out)
		}
	}
	point := mount(t, output)

	// The application and the link people drag it onto.
	if got, err := os.ReadFile(filepath.Join(point, "Demo.app", "Contents", "MacOS", "Demo")); err != nil {
		t.Fatal(err)
	} else if string(got) != "#!/bin/sh\n" {
		t.Fatalf("the application binary came back as %q", got)
	}
	if target, err := os.Readlink(filepath.Join(point, "Applications")); err != nil || target != "/Applications" {
		t.Fatalf("Applications link: %q %v", target, err)
	}

	// A framework's links must stay links: its code signature covers the shape
	// of the bundle, so following them would both bloat and break it.
	framework := filepath.Join(point, "Demo.app", "Contents", "Frameworks", "X.framework")
	for path, want := range map[string]string{
		filepath.Join(framework, "X"):                   "Versions/Current/X",
		filepath.Join(framework, "Versions", "Current"): "A",
	} {
		got, err := os.Readlink(path)
		if err != nil {
			t.Fatalf("%s is not a link: %v", path, err)
		}
		if got != want {
			t.Fatalf("%s points at %q, want %q", path, got, want)
		}
	}

	// The executable bit has to survive, or the application will not launch.
	info, err := os.Stat(filepath.Join(point, "Demo.app", "Contents", "MacOS", "Demo"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("the application binary is not executable: %v", info.Mode())
	}

	// The window settings and the artwork they refer to.
	for _, name := range []string{".DS_Store", ".VolumeIcon.icns", ".background/background.png"} {
		if _, err := os.Stat(filepath.Join(point, name)); err != nil {
			t.Fatal(err)
		}
	}
}

// TestBackgroundAliasMatchesTheMountedVolume is the check that the whole
// mount-free approach rests on: the alias record naming the background image is
// built from catalog node IDs worked out before the image existed, and those
// have to be the numbers the volume really reports once it is mounted.
func TestBackgroundAliasMatchesTheMountedVolume(t *testing.T) {
	dir := t.TempDir()
	background := filepath.Join(dir, "bg.png")
	if err := os.WriteFile(background, []byte("background"), 0644); err != nil {
		t.Fatal(err)
	}
	payload := filepath.Join(dir, "readme.txt")
	if err := os.WriteFile(payload, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	output := filepath.Join(dir, "Aliased.dmg")
	err := CreateDMG(context.Background(), Config{
		FileName: output, Title: "Aliased", Background: background,
		ContentsIconSize: 128, WindowWidth: 640, WindowHeight: 480,
		Contents: []Item{{X: 10, Y: 20, Type: File, Path: payload}},
	})
	if err != nil {
		t.Fatal(err)
	}
	point := mount(t, output)

	store, err := os.ReadFile(filepath.Join(point, ".DS_Store"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".background", ".background/background.png"} {
		info, err := os.Stat(filepath.Join(point, name))
		if err != nil {
			t.Fatal(err)
		}
		// The inode a mounted HFS+ volume reports is the catalog node ID the
		// image was built with, and the alias record holds it big endian.
		id := uint32(info.Sys().(*syscall.Stat_t).Ino)
		if !bytes.Contains(store, binary.BigEndian.AppendUint32(nil, id)) {
			t.Fatalf("the window settings do not refer to %s by its catalog node ID %d", name, id)
		}
	}
}

func TestRejectsBadConfig(t *testing.T) {
	dir := t.TempDir()
	for name, config := range map[string]Config{
		"no title":         {FileName: filepath.Join(dir, "a.dmg")},
		"unknown type":     {Title: "T", FileName: filepath.Join(dir, "b.dmg"), Contents: []Item{{Type: "sideways", Path: "x"}}},
		"missing file":     {Title: "T", FileName: filepath.Join(dir, "c.dmg"), Contents: []Item{{Type: File, Path: filepath.Join(dir, "gone")}}},
		"missing dir":      {Title: "T", FileName: filepath.Join(dir, "d.dmg"), Contents: []Item{{Type: Dir, Path: filepath.Join(dir, "gone")}}},
		"missing backdrop": {Title: "T", FileName: filepath.Join(dir, "e.dmg"), Background: filepath.Join(dir, "gone.png")},
	} {
		t.Run(name, func(t *testing.T) {
			if err := CreateDMG(context.Background(), config); err == nil {
				t.Fatal("expected an error")
			}
			if config.FileName != "" {
				if _, err := os.Stat(config.FileName); !os.IsNotExist(err) {
					t.Fatal("a failed build left an image behind")
				}
			}
		})
	}
}

func TestFileNameGainsExtension(t *testing.T) {
	dir := t.TempDir()
	config := Config{
		FileName: filepath.Join(dir, "NoExtension"), Title: "T",
		ContentsIconSize: 128, WindowWidth: 640, WindowHeight: 480,
	}
	if err := CreateDMG(context.Background(), config); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(config.FileName + ".dmg"); err != nil {
		t.Fatal(err)
	}
}
