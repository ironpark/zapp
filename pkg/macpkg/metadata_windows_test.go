package macpkg

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsPayloadMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file")
	link := filepath.Join(dir, "hardlink")
	if err := os.WriteFile(path, []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(path, link); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	a, err := payloadMetadata(path, info)
	if err != nil {
		t.Fatal(err)
	}
	b, err := payloadMetadata(link, info)
	if err != nil {
		t.Fatal(err)
	}
	if a.Dev != b.Dev || a.Ino != b.Ino {
		t.Fatal("hard links have different identities")
	}
	if a.Mode != 0100644 || a.Uid != 0 || a.Gid != 0 {
		t.Fatalf("invalid Unix metadata: %+v", a)
	}
	// Windows keeps no execute bits; an app's executable gets them back.
	exe := filepath.Join(dir, "MacOS", "Demo")
	if err := os.MkdirAll(filepath.Dir(exe), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("launcher"), 0644); err != nil {
		t.Fatal(err)
	}
	if info, err = os.Stat(exe); err != nil {
		t.Fatal(err)
	}
	if s, err := payloadMetadata(exe, info); err != nil || s.Mode != 0100755 {
		t.Fatalf("executable: %+v, %v", s, err)
	}
	// Windows has no Unix ownership, and collect rejects it before the walk.
	if _, err := collect(context.Background(), dir, "", PreserveOwnership); err == nil {
		t.Fatal("accepted Unix ownership preservation")
	}
}
