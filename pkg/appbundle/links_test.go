package appbundle

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// framework lays out Lib.framework in app with its links made by link.
func framework(t *testing.T, app string, link func(target, name string)) {
	t.Helper()
	fw := filepath.Join(app, "Contents", "Frameworks", "Lib.framework")
	if err := os.MkdirAll(filepath.Join(fw, "Versions", "A", "Resources"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fw, "Versions", "A", "Lib"), []byte("\xcf\xfa\xed\xfe binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	link("A", filepath.Join(fw, "Versions", "Current"))
	link("Versions/Current/Lib", filepath.Join(fw, "Lib"))
	link("Versions/Current/Resources", filepath.Join(fw, "Resources"))
}

func TestCheckLinks(t *testing.T) {
	write := func(t *testing.T) func(string, string) {
		return func(target, name string) {
			if err := os.WriteFile(name, []byte(target), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Git on Windows without core.symlinks.
	app := filepath.Join(t.TempDir(), "Demo.app")
	framework(t, app, write(t))
	err := CheckLinks(app)
	if err == nil || !strings.Contains(err.Error(), `Contents/Frameworks/Lib.framework/Versions/Current is a text file holding "A"`) || !strings.Contains(err.Error(), "core.symlinks") {
		t.Fatalf("text files: %v", err)
	}

	// A copy that followed the links.
	app = filepath.Join(t.TempDir(), "Demo.app")
	framework(t, app, func(target, name string) {
		if filepath.Base(name) == "Current" {
			if err := os.MkdirAll(filepath.Join(name, "Resources"), 0o755); err != nil {
				t.Fatal(err)
			}
			return
		}
		write(t)(strings.Repeat("\x00", 2048), name)
	})
	err = CheckLinks(app)
	if err == nil || !strings.Contains(err.Error(), "Versions/Current is a copy of the directory") || !strings.Contains(err.Error(), "Lib.framework/Lib is a copy of the file") || strings.Contains(err.Error(), "core.symlinks") {
		t.Fatalf("copies: %v", err)
	}

	// A shallow framework has no links to lose.
	app = filepath.Join(t.TempDir(), "Demo.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents", "Frameworks", "Flat.framework"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := CheckLinks(app); err != nil {
		t.Fatal(err)
	}

	if runtime.GOOS == "windows" {
		return // making links needs Developer Mode
	}
	framework(t, app, func(target, name string) {
		if err := os.Symlink(target, name); err != nil {
			t.Fatal(err)
		}
	})
	if err := CheckLinks(app); err != nil {
		t.Fatal(err)
	}
}
