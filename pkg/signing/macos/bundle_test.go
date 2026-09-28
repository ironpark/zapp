package macos

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

var thinMachO = []byte{0xcf, 0xfa, 0xed, 0xfe, 7, 0, 0, 1}

func TestNestedCode(t *testing.T) {
	app := filepath.Join(t.TempDir(), "Demo.app")
	file := func(rel string, data []byte) {
		t.Helper()
		path := filepath.Join(app, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	file("Contents/Info.plist", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict><key>CFBundleExecutable</key><string>demo</string></dict></plist>`))
	file("Contents/MacOS/demo", thinMachO)
	file("Contents/MacOS/tool", thinMachO)
	file("Contents/Frameworks/libfoo.dylib", thinMachO)
	file("Contents/Frameworks/Foo.framework/Versions/A/Foo", thinMachO)
	file("Contents/Frameworks/Foo.framework/Versions/A/Libraries/libbar.dylib", thinMachO)
	if err := os.Symlink("A", filepath.Join(app, "Contents/Frameworks/Foo.framework/Versions/Current")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("Versions/Current/Foo", filepath.Join(app, "Contents/Frameworks/Foo.framework/Foo")); err != nil {
		t.Fatal(err)
	}
	file("Contents/Frameworks/Helper.app/Contents/MacOS/Helper", thinMachO)
	file("Contents/Resources/Assets.bundle/Contents/Resources/image.png", []byte("not code"))
	file("Contents/Resources/script.sh", []byte("#!/bin/sh\n"))
	file("Contents/Resources/Main.class", []byte{0xca, 0xfe, 0xba, 0xbe, 0, 0, 0, 52})
	file("Contents/Resources/universal", []byte{0xca, 0xfe, 0xba, 0xbe, 0, 0, 0, 2})

	groups, err := nestedCode(app)
	if err != nil {
		t.Fatal(err)
	}
	for i, g := range groups {
		for j, p := range g {
			rel, _ := filepath.Rel(app, p)
			groups[i][j] = filepath.ToSlash(rel)
		}
	}
	want := [][]string{
		{"Contents/Frameworks/Foo.framework/Versions/A/Libraries/libbar.dylib"},
		{"Contents/Frameworks/Foo.framework", "Contents/Frameworks/Helper.app", "Contents/Frameworks/libfoo.dylib", "Contents/MacOS/tool", "Contents/Resources/universal"},
	}
	if !reflect.DeepEqual(groups, want) {
		t.Errorf("nestedCode() = %q, want %q", groups, want)
	}
}
