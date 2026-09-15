package macpkg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Never run installation tests on a developer's host. Both explicit opt-in and
// a hypervisor indication are required, and the VM must be disposable.
func TestInstallInDisposableVM(t *testing.T) {
	if os.Getenv("MACPKG_INSTALL_TEST") != "1" {
		t.Skip("requires explicit opt-in in a disposable macOS VM")
	}
	if runtime.GOOS != "darwin" || os.Geteuid() != 0 {
		t.Fatal("installation test requires root in a macOS VM")
	}
	vm, err := exec.Command("sysctl", "-n", "kern.hv_vmm_present").Output()
	if err != nil || strings.TrimSpace(string(vm)) != "1" {
		t.Fatal("refusing installation outside a detected VM")
	}
	dir := t.TempDir()
	root := filepath.Join(dir, "root")
	app := filepath.Join(root, "Test.app")
	scripts := filepath.Join(dir, "scripts")
	installed := filepath.Join(dir, "installed")
	id := fmt.Sprintf("dev.zapp.install-test.%d", time.Now().UnixNano())
	defer exec.Command("pkgutil", "--forget", id).Run()
	put(t, filepath.Join(scripts, "preinstall"), []byte("#!/bin/sh\nmkdir -p \"$2\"\nprintf pre > \"$2/preinstall-ran\"\n"), 0755)
	put(t, filepath.Join(scripts, "postinstall"), []byte("#!/bin/sh\nprintf post > \"$2/postinstall-ran\"\n"), 0755)
	put(t, filepath.Join(app, "Contents", "old"), []byte("remove on upgrade"), 0644)
	for _, v := range []string{"1", "2"} {
		metadata := fmt.Sprintf(`<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleIdentifier</key><string>%s.app</string><key>CFBundleVersion</key><string>%s</string></dict></plist>`, id, v)
		put(t, filepath.Join(app, "Contents", "Info.plist"), []byte(metadata), 0644)
		put(t, filepath.Join(app, "Contents", "version"), []byte(v), 0644)
		if v == "2" {
			if err := os.Remove(filepath.Join(app, "Contents", "old")); err != nil {
				t.Fatal(err)
			}
		}
		component := filepath.Join(dir, "component.pkg")
		if err := BuildComponent(context.Background(), ComponentConfig{Root: root, Identifier: id, Version: v, InstallLocation: installed, ScriptsDir: scripts, OutputPath: component}); err != nil {
			t.Fatal(err)
		}
		product := filepath.Join(dir, "product.pkg")
		if err := BuildProduct(context.Background(), ProductConfig{Packages: []string{component}, OutputPath: product}); err != nil {
			t.Fatal(err)
		}
		native(t, "installer", "-pkg", product, "-target", "/")
		for p, want := range map[string]string{"Test.app/Contents/version": v, "preinstall-ran": "pre", "postinstall-ran": "post"} {
			b, err := os.ReadFile(filepath.Join(installed, p))
			if err != nil || string(b) != want {
				t.Fatalf("%s: %q, %v", p, b, err)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(installed, "Test.app", "Contents", "old")); !os.IsNotExist(err) {
		t.Fatal("upgrade retained removed app file")
	}
}

func TestProductsignCompatibility(t *testing.T) {
	identity := os.Getenv("MACPKG_SIGN_IDENTITY")
	if identity == "" {
		t.Skip("requires a configured Developer ID Installer identity")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("productsign requires macOS")
	}
	dir, c := fixture(t)
	if err := BuildComponent(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	product := filepath.Join(dir, "product.pkg")
	if err := BuildProduct(context.Background(), ProductConfig{Packages: []string{c.OutputPath}, OutputPath: product}); err != nil {
		t.Fatal(err)
	}
	signed := filepath.Join(dir, "signed.pkg")
	native(t, "productsign", "--sign", identity, "--timestamp=none", product, signed)
	native(t, "pkgutil", "--check-signature", signed)
	native(t, "pkgutil", "--expand-full", signed, filepath.Join(dir, "expanded"))
}
