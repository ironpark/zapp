//go:build cgo && (linux || windows) && (amd64 || arm64)

package signing

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// rcodesign signs with a PKCS#12 certificate in the modern encryption that its
// own reader rejects as a wrong password.
func TestRcodesignSignsWithModernP12(t *testing.T) {
	dir := t.TempDir()
	src, bin, file := filepath.Join(dir, "main.go"), filepath.Join(dir, "demo"), filepath.Join(dir, "cert.p12")
	if err := os.WriteFile(src, []byte("package main\n\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-o", bin, src)
	build.Env = append(os.Environ(), "GOOS=darwin", "GOARCH=arm64", "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building a Mach-O: %v\n%s", err, out)
	}
	if err := os.WriteFile(file, testP12(t, "secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := Select(Credentials{P12File: file, P12Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = Close(b) }()
	if err := b.Sign(t.Context(), bin); err != nil {
		t.Fatal(err)
	}
}
