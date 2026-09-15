package installer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCreatePKG(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "App & 한글.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents"), 0755); err != nil {
		t.Fatal(err)
	}
	plist := `<plist version="1.0"><dict><key>CFBundleIdentifier</key><string>dev.zapp.app</string><key>CFBundleVersion</key><string>1</string></dict></plist>`
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte(plist), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unrelated-secret"), []byte("must not be shipped"), 0644); err != nil {
		t.Fatal(err)
	}
	license := filepath.Join(dir, "license.txt")
	if err := os.WriteFile(license, []byte("License"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, withLicense := range []bool{false, true} {
		t.Run(map[bool]string{false: "no license", true: "localized license"}[withLicense], func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "app.pkg")
			c := Config{AppPath: app + string(filepath.Separator), OutputPath: output, Identifier: "dev.zapp.app", Version: "1", InstallLocation: "/Applications"}
			if withLicense {
				c.LicensePaths = map[string]string{"EN": license, "ko": license}
			}
			if err := CreatePKG(context.Background(), c); err != nil {
				t.Fatal(err)
			}
			if runtime.GOOS != "darwin" {
				return
			}
			expanded := filepath.Join(t.TempDir(), "expanded")
			b, err := exec.Command("pkgutil", "--expand-full", output, expanded).CombinedOutput()
			if err != nil {
				t.Fatalf("expand: %v %s", err, b)
			}
			payload := filepath.Join(expanded, "component.pkg", "Payload")
			entries, err := os.ReadDir(payload)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || entries[0].Name() != filepath.Base(app) {
				t.Fatalf("unexpected payload: %v", entries)
			}
			d, err := os.ReadFile(filepath.Join(expanded, "Distribution"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(d), "<license ") != withLicense {
				t.Fatalf("bad license declaration: %s", d)
			}
			if withLicense {
				if _, err := os.Stat(filepath.Join(expanded, "Resources", "en.lproj", "license.txt")); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
