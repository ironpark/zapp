package macpkg

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildApp(t *testing.T) {
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
			c := AppConfig{AppPath: app + string(filepath.Separator), OutputPath: output, Identifier: "dev.zapp.app", Version: "1"}
			if withLicense {
				c.Licenses = map[string]string{"EN": license, "ko": license}
			}
			if err := BuildApp(context.Background(), c); err != nil {
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

func TestBuildAppLicenseFallbackAndDefaults(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "My App.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte(`<plist version="1.0"><dict><key>CFBundleIdentifier</key><string>dev.zapp.app</string><key>CFBundleVersion</key><string>1</string></dict></plist>`), 0644); err != nil {
		t.Fatal(err)
	}
	license := filepath.Join(dir, "license.txt")
	if err := os.WriteFile(license, []byte("License"), 0644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "app.pkg")
	c := AppConfig{AppPath: app, OutputPath: output, Identifier: "dev.zapp.app", Version: "1", License: license, Licenses: map[string]string{"ko": license}}
	if err := BuildApp(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "darwin" {
		return
	}
	expanded := filepath.Join(t.TempDir(), "expanded")
	if b, err := exec.Command("pkgutil", "--expand-full", output, expanded).CombinedOutput(); err != nil {
		t.Fatalf("expand: %v %s", err, b)
	}
	// The fallback sits at the Resources root; the localized copy in its .lproj.
	for _, rel := range []string{"license.txt", filepath.Join("ko.lproj", "license.txt")} {
		if _, err := os.Stat(filepath.Join(expanded, "Resources", rel)); err != nil {
			t.Fatal(err)
		}
	}
	d, err := os.ReadFile(filepath.Join(expanded, "Distribution"))
	if err != nil {
		t.Fatal(err)
	}
	// Title defaults to the app name without .app, and install goes to /Applications.
	if !strings.Contains(string(d), "<title>My App</title>") {
		t.Fatalf("missing default title: %s", d)
	}
	b, err := os.ReadFile(filepath.Join(expanded, "component.pkg", "PackageInfo"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `install-location="/Applications"`) {
		t.Fatalf("unexpected install location: %s", b)
	}
}

func TestBuildAppRejectsBadInput(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "App.app")
	if err := os.MkdirAll(app, 0755); err != nil {
		t.Fatal(err)
	}
	base := AppConfig{AppPath: app, OutputPath: filepath.Join(dir, "out.pkg"), Identifier: "dev.zapp.app", Version: "1"}
	for name, mutate := range map[string]func(*AppConfig){
		"no app path":     func(c *AppConfig) { c.AppPath = "" },
		"missing app":     func(c *AppConfig) { c.AppPath = filepath.Join(dir, "absent.app") },
		"app is a file":   func(c *AppConfig) { c.AppPath = writeTemp(t, dir, "file.app") },
		"bad language":    func(c *AppConfig) { c.Licenses = map[string]string{"klingon": "x"} },
		"dup language":    func(c *AppConfig) { c.Licenses = map[string]string{"ko": "x", "KO": "y"} },
		"no license path": func(c *AppConfig) { c.Licenses = map[string]string{"ko": ""} },
	} {
		t.Run(name, func(t *testing.T) {
			c := base
			mutate(&c)
			if err := BuildApp(context.Background(), c); err == nil {
				t.Fatal("expected an error")
			}
			if _, err := os.Stat(c.OutputPath); !os.IsNotExist(err) {
				t.Fatal("output should not exist")
			}
		})
	}
}

func writeTemp(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}
