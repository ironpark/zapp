package macpkg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A component written to another drive than its root, as a temporary
// directory on C: is to a checkout on D: on GitHub's Windows runners, is
// outside the root.
func TestOutputOnAnotherDrive(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "component.pkg")
	if strings.EqualFold(filepath.VolumeName(root), filepath.VolumeName(out)) {
		t.Skip("the working and temporary directories share a drive")
	}
	if err := outputOutside(out, root, ""); err != nil {
		t.Fatal(err)
	}
}
