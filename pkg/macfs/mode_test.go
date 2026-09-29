package macfs

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// What Windows reports, 0666 for every file, becomes what a Mac expects.
func TestUnixMode(t *testing.T) {
	dir := t.TempDir()
	files := map[string]struct {
		data string
		want fs.FileMode
	}{
		"Demo.app/Contents/MacOS/Demo":                              {"launcher", 0o755},
		"Demo.app/Contents/Helpers/tool":                            {"\xcf\xfa\xed\xfe\x07\x00\x00\x01", 0o755},
		"Demo.app/Contents/Frameworks/Lib.framework/Versions/A/Lib": {"\xca\xfe\xba\xbe\x00\x00\x00\x02", 0o755},
		"Demo.app/Contents/Resources/run.sh":                        {"#!/bin/sh\n", 0o755},
		"Demo.app/Contents/Resources/Main.class":                    {"\xca\xfe\xba\xbe\x00\x00\x00\x41", 0o644},
		"Demo.app/Contents/Info.plist":                              {"<plist/>", 0o644},
		"Demo.app/Contents/PkgInfo":                                 {"APPL", 0o644},
	}
	for name, f := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(f.data), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	for name, f := range files {
		path := filepath.Join(dir, name)
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := unixMode(path, info); err != nil || got != f.want {
			t.Errorf("%s: %v, %v; want %v", name, got, err, f.want)
		}
	}
	info, err := os.Lstat(filepath.Join(dir, "Demo.app"))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := unixMode(dir, info); got != fs.ModeDir|0o755 {
		t.Errorf("directory: %v", got)
	}
}
