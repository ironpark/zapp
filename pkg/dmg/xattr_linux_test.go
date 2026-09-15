package dmg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLargeHostIconDoesNotPreventLinuxBuild(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.dmg")
	if err := os.WriteFile(path, []byte("image"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := applyImageIcon(path, make([]byte, 2<<20)); err != nil {
		t.Fatal(err)
	}
}
