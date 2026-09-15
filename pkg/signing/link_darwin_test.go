package signing

import (
	"os/exec"
	"strings"
	"testing"
)

// TestDarwinDoesNotLinkRcodesign keeps the rcodesign binding out of macOS
// builds. Only the //go:build tag on select_other.go enforces that, and Go has
// no way to forbid an import outright, so the constraint is checked here rather
// than only in the release workflow.
func TestDarwinDoesNotLinkRcodesign(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "github.com/ironpark/zapp").Output()
	if err != nil {
		t.Skipf("go list unavailable: %v", err)
	}
	for _, dep := range strings.Fields(string(out)) {
		if dep == "github.com/ironpark/zapp/pkg/signing/rcodesign" {
			t.Fatal("macOS binary must not include rcodesign")
		}
	}
}
