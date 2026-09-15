package dmg

import (
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
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

func TestHostIconErrors(t *testing.T) {
	for _, errno := range []error{unix.E2BIG, unix.ENOTSUP, unix.ENOSPC} {
		if err := hostIconResult(fmt.Errorf("write resource fork: %w", errno)); err != nil {
			t.Fatalf("optional metadata error: %v", err)
		}
	}
	for _, errno := range []error{unix.EACCES, unix.EIO, unix.ENOENT} {
		if err := hostIconResult(fmt.Errorf("write resource fork: %w", errno)); !errors.Is(err, errno) {
			t.Fatalf("lost error %v: %v", errno, err)
		}
	}
	if err := hostIconResult(nil); err != nil {
		t.Fatal(err)
	}
}
