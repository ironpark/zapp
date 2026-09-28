package archive

import (
	"archive/zip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// An app bundle round-trips: names start at the bundle, the executable keeps
// its permissions and a framework's Current link stays a link.
func TestZipKeepsBundleStructure(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "Demo.app")
	framework := filepath.Join(app, "Contents", "Frameworks", "Lib.framework")
	if err := os.MkdirAll(filepath.Join(framework, "Versions", "A"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "Contents", "MacOS", "Demo"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(framework, "Versions", "A", "Lib"), []byte("binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	symlinks := runtime.GOOS != "windows"
	if symlinks {
		if err := os.Symlink("A", filepath.Join(framework, "Versions", "Current")); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(dir, "dist", "Demo.zip")
	if err := Zip(t.Context(), app, out); err != nil {
		t.Fatal(err)
	}
	r, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	entries := map[string]*zip.File{}
	for _, f := range r.File {
		entries[f.Name] = f
	}
	for _, name := range []string{"Demo.app/", "Demo.app/Contents/MacOS/Demo", "Demo.app/Contents/Frameworks/Lib.framework/Versions/A/Lib"} {
		if entries[name] == nil {
			t.Fatalf("missing %s in %v", name, entries)
		}
	}
	if runtime.GOOS != "windows" && entries["Demo.app/Contents/MacOS/Demo"].Mode()&0o111 == 0 {
		t.Fatal("executable lost its permissions")
	}
	if symlinks {
		link := entries["Demo.app/Contents/Frameworks/Lib.framework/Versions/Current"]
		if link == nil || link.Mode()&fs.ModeSymlink == 0 {
			t.Fatal("Current was not stored as a link")
		}
		rc, err := link.Open()
		if err != nil {
			t.Fatal(err)
		}
		target, _ := io.ReadAll(rc)
		_ = rc.Close()
		if string(target) != "A" {
			t.Fatalf("link target = %q", target)
		}
	}
	leftovers, _ := filepath.Glob(filepath.Join(dir, "dist", ".*"))
	if len(leftovers) != 0 {
		t.Fatalf("temporary files left: %v", leftovers)
	}
}

func TestZipReportsMissingSource(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out.zip")
	if err := Zip(t.Context(), filepath.Join(dir, "missing"), out); !os.IsNotExist(err) {
		t.Fatalf("missing source: %v", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("an archive was left behind")
	}
}
