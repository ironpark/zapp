//go:build cgo && (linux || windows) && (amd64 || arm64)

package rcodesign

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticBindingWithoutExecutable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := Available(); err != nil {
		t.Fatal(err)
	}
	b := New(Options{PEMFile: filepath.Join(t.TempDir(), "missing.pem")})
	err := b.Sign(t.Context(), filepath.Join(t.TempDir(), "Demo.app"))
	if err == nil || errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected Rust credential error, got %v", err)
	}
	if err := invoke(t.Context(), "unknown", "target", Options{}); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("C ABI did not return the Rust validation error: %v", err)
	}
}

func TestCancelledCallDoesNotEnterRust(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := invoke(ctx, "unknown", "target", Options{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want cancellation", err)
	}
}

// An unsigned binary fails verification with its path in the bundle.
func TestVerifyReportsUnsignedCode(t *testing.T) {
	app := filepath.Join(t.TempDir(), "Demo.app")
	exe := filepath.Join(app, "Contents", "MacOS", "Demo")
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	header := make([]byte, 32)
	binary.LittleEndian.PutUint32(header, 0xfeedfacf)
	binary.LittleEndian.PutUint32(header[4:], 0x0100000c)
	binary.LittleEndian.PutUint32(header[12:], 2)
	if err := os.WriteFile(exe, header, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Verify(t.Context(), app); err == nil || !strings.Contains(err.Error(), "Contents/MacOS/Demo:") {
		t.Fatalf("unsigned: %v", err)
	}
}
