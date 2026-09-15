package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, content string, mode os.FileMode) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	// WriteFile applies the umask, so set the mode explicitly.
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCopyFilePreservesMode(t *testing.T) {
	dir := t.TempDir()
	src := write(t, filepath.Join(dir, "App"), "#!/bin/sh\n", 0755)
	dst := filepath.Join(dir, "nested", "deeper", "App")

	if err := CopyFile(src, dst); err != nil {
		t.Fatalf("CopyFile() error: %v", err)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("destination not created: %v", err)
	}
	// An executable that loses its exec bit cannot be launched from the copy.
	if info.Mode().Perm() != 0755 {
		t.Errorf("mode = %v, want -rwxr-xr-x", info.Mode().Perm())
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "#!/bin/sh\n" {
		t.Errorf("contents = %q", got)
	}
}

func TestCopyFileOverwrites(t *testing.T) {
	dir := t.TempDir()
	src := write(t, filepath.Join(dir, "src"), "new", 0644)
	dst := write(t, filepath.Join(dir, "dst"), "old and much longer", 0644)

	if err := CopyFile(src, dst); err != nil {
		t.Fatalf("CopyFile() error: %v", err)
	}
	got, _ := os.ReadFile(dst)
	// Without O_TRUNC the tail of the old contents would survive.
	if string(got) != "new" {
		t.Errorf("contents = %q, want %q", got, "new")
	}
}

func TestCopyFileRejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := CopyFile(dir, filepath.Join(dir, "out")); err == nil {
		t.Error("CopyFile() on a directory returned no error")
	}
}

func TestCopyDir(t *testing.T) {
	src := t.TempDir()
	write(t, filepath.Join(src, "Contents", "MacOS", "App"), "bin", 0755)
	write(t, filepath.Join(src, "Contents", "Info.plist"), "plist", 0644)
	write(t, filepath.Join(src, "Contents", "Resources", "icon.icns"), "icns", 0600)

	dst := filepath.Join(t.TempDir(), "Copy.app")
	if err := CopyDir(src, dst); err != nil {
		t.Fatalf("CopyDir() error: %v", err)
	}

	for rel, wantMode := range map[string]os.FileMode{
		"Contents/MacOS/App":           0755,
		"Contents/Info.plist":          0644,
		"Contents/Resources/icon.icns": 0600,
	} {
		info, err := os.Stat(filepath.Join(dst, rel))
		if err != nil {
			t.Errorf("%s missing from copy: %v", rel, err)
			continue
		}
		if info.Mode().Perm() != wantMode {
			t.Errorf("%s mode = %v, want %v", rel, info.Mode().Perm(), wantMode)
		}
	}
}

func TestCopyDirRejectsFile(t *testing.T) {
	dir := t.TempDir()
	src := write(t, filepath.Join(dir, "file"), "x", 0644)
	if err := CopyDir(src, filepath.Join(dir, "out")); err == nil {
		t.Error("CopyDir() on a regular file returned no error")
	}
}

func TestIsDir(t *testing.T) {
	dir := t.TempDir()
	file := write(t, filepath.Join(dir, "file"), "x", 0644)

	if got, err := IsDir(dir); err != nil || !got {
		t.Errorf("IsDir(dir) = %v, %v; want true, nil", got, err)
	}
	if got, err := IsDir(file); err != nil || got {
		t.Errorf("IsDir(file) = %v, %v; want false, nil", got, err)
	}

	// A symlink is reported as whatever it points at.
	link := filepath.Join(dir, "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	if got, err := IsDir(link); err != nil || !got {
		t.Errorf("IsDir(symlink to dir) = %v, %v; want true, nil", got, err)
	}

	if _, err := IsDir(filepath.Join(dir, "missing")); err == nil {
		t.Error("IsDir() on a missing path returned no error")
	}
}

func TestSameFile(t *testing.T) {
	dir := t.TempDir()
	a := write(t, filepath.Join(dir, "a"), "x", 0644)
	b := write(t, filepath.Join(dir, "b"), "x", 0644)

	if got, err := SameFile(a, a); err != nil || !got {
		t.Errorf("SameFile(a, a) = %v, %v; want true, nil", got, err)
	}
	// Identical contents are still different files.
	if got, err := SameFile(a, b); err != nil || got {
		t.Errorf("SameFile(a, b) = %v, %v; want false, nil", got, err)
	}

	link := filepath.Join(dir, "link")
	if err := os.Symlink(a, link); err != nil {
		t.Fatal(err)
	}
	if got, err := SameFile(a, link); err != nil || !got {
		t.Errorf("SameFile(a, symlink to a) = %v, %v; want true, nil", got, err)
	}

	// A missing destination is the guarded case: not the same, not an error.
	if got, err := SameFile(a, filepath.Join(dir, "missing")); err != nil || got {
		t.Errorf("SameFile(a, missing) = %v, %v; want false, nil", got, err)
	}
	if _, err := SameFile(filepath.Join(dir, "missing"), a); err == nil {
		t.Error("SameFile() with a missing source returned no error")
	}
}
