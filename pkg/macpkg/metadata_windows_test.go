package macpkg

import (
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
	a, err := payloadMetadata(path, info, RootWheel)
	if err != nil {
		t.Fatal(err)
	}
	b, err := payloadMetadata(link, info, RootWheel)
	if err != nil {
		t.Fatal(err)
	}
	if a.Dev != b.Dev || a.Ino != b.Ino {
		t.Fatal("hard links have different identities")
	}
	if a.Mode&0170000 != 0100000 || a.Uid != 0 || a.Gid != 0 {
		t.Fatalf("invalid Unix metadata: %+v", a)
	}
	if _, err := payloadMetadata(path, info, PreserveOwnership); err == nil {
		t.Fatal("accepted Unix ownership preservation")
	}
}
