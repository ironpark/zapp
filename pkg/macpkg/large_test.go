package macpkg

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// This test writes and expands a real >8 GiB file. It is opt-in because Apple
// expansion allocates disk space even when the source file is sparse.
func TestLargeFileIntegration(t *testing.T) {
	if os.Getenv("MACPKG_LARGE_TEST") != "1" {
		t.Skip("set MACPKG_LARGE_TEST=1; requires 10 GiB free disk")
	}
	dir := t.TempDir()
	root := filepath.Join(dir, "root")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "big")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	const size = legacyLimit + 37
	if err = f.Truncate(size); err != nil {
		t.Fatal(err)
	}
	// Nonzero bytes on both sides of segment boundaries catch reorder/truncation.
	for _, off := range []int64{0, segmentSize - 1, segmentSize, legacyLimit - 1, legacyLimit, size - 1} {
		if _, err = f.WriteAt([]byte{byte(off%251 + 1)}, off); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.Link(p, filepath.Join(root, "hard")); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "large.pkg")
	c := ComponentConfig{Root: root, Identifier: "dev.zapp.large-test", Version: "1", OutputPath: output}
	if err = BuildComponent(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "darwin" {
		t.Skip("large build passed; native expansion requires macOS")
	}
	expanded := filepath.Join(dir, "expanded")
	native(t, "pkgutil", "--expand-full", output, expanded)
	restored := filepath.Join(expanded, "LargeSegmentedPayload", "big")
	st, err := os.Stat(restored)
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() != size {
		t.Fatalf("size %d, expected %d", st.Size(), size)
	}
	hard, err := os.Stat(filepath.Join(expanded, "LargeSegmentedPayload", "hard"))
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(st, hard) {
		t.Fatal("large hard link not preserved")
	}
	digest := func(p string) string {
		f, err := os.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = f.Close() }()
		h := sha256.New()
		if _, err = io.Copy(h, f); err != nil {
			t.Fatal(err)
		}
		return hex.EncodeToString(h.Sum(nil))
	}
	if digest(p) != digest(restored) {
		t.Fatal("large file digest mismatch")
	}
	listing := native(t, "lsbom", "-p", "sf", filepath.Join(expanded, "Bom"))
	if !strings.Contains(listing, strconv.FormatInt(size, 10)) {
		t.Fatalf("Size64 lost: %s", listing)
	}
	product := filepath.Join(dir, "product.pkg")
	if err = BuildProduct(context.Background(), ProductConfig{OutputPath: product, Packages: []string{output}}); err != nil {
		t.Fatal(err)
	}
	a, err := openXAR(context.Background(), product)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.f.Close() }()
	b, err := a.read(context.Background(), "Distribution", maxMetadata)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `min="12.0"`) {
		t.Fatal("product lost minimum OS")
	}
}

// A Linux test binary can write these products to a bind-mounted directory for
// subsequent pkgutil/installer validation on the macOS host.
func TestCrossPlatformArtifact(t *testing.T) {
	dir := os.Getenv("MACPKG_ARTIFACT_DIR")
	if dir == "" {
		t.Skip("set MACPKG_ARTIFACT_DIR to export a platform fixture")
	}
	root := filepath.Join(t.TempDir(), "root")
	put(t, filepath.Join(root, "hello"), []byte("hello from "+runtime.GOOS), 0755)
	c := ComponentConfig{Root: root, Identifier: "dev.zapp.cross-platform", Version: "1", OutputPath: filepath.Join(dir, "component.pkg")}
	if err := BuildComponent(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if err := BuildProduct(context.Background(), ProductConfig{OutputPath: filepath.Join(dir, "product.pkg"), Packages: []string{c.OutputPath}}); err != nil {
		t.Fatal(err)
	}
}
