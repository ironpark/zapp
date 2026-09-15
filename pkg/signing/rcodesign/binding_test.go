//go:build cgo && (linux || windows) && (amd64 || arm64)

package rcodesign

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticBindingWithoutExecutable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := Available(); err != nil {
		t.Fatal(err)
	}
	b := New(Options{P12File: filepath.Join(t.TempDir(), "missing.p12")})
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
