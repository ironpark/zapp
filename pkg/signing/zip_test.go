package signing

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateZip(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "App.app")
	if err := os.MkdirAll(filepath.Join(root, "Contents"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Contents", "data"), []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "app.zip")
	if err := createZip(root, out); err != nil {
		t.Fatal(err)
	}
	r, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	f, err := r.Open("App.app/Contents/data")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(f)
	if err != nil || string(b) != "payload" {
		t.Fatalf("ZIP payload: %q, %v", b, err)
	}
}

func TestCreateZipReportsSourceErrors(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out.zip")
	if err := createZip(filepath.Join(dir, "missing"), out); !os.IsNotExist(err) {
		t.Fatalf("missing source: %v", err)
	}
	root := filepath.Join(dir, "root")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing", filepath.Join(root, "dangling")); err != nil {
		t.Fatal(err)
	}
	if err := createZip(root, out); err == nil {
		t.Fatal("ignored error while reading a walked file")
	}
}
