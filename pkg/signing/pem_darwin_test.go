//go:build darwin

package signing

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The PKCS#12 made for Apple's tools is one `security import` accepts, and the imported key
// pairs with its certificate as a code signing identity. The keychain is
// created and deleted here without touching the user's search list.
func TestPEMImportsIntoKeychain(t *testing.T) {
	if os.Getenv("ZAPP_KEYCHAIN_TEST") == "" {
		t.Skip("creates a keychain; set ZAPP_KEYCHAIN_TEST=1 to run")
	}
	key := ecKey(t)
	p12, password, err := p12Bundle(key, testCert(t, "Zapp PEM Signing", key, nil, nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	keychain, p12Path := filepath.Join(dir, "test.keychain-db"), filepath.Join(dir, "cert.p12")
	if err := os.WriteFile(p12Path, p12, 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("security", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("security %s: %v\n%s", args[0], err, out)
		}
		return string(out)
	}
	run("create-keychain", "-p", "test", keychain)
	t.Cleanup(func() { _ = exec.Command("security", "delete-keychain", keychain).Run() })
	run("unlock-keychain", "-p", "test", keychain)
	run("import", p12Path, "-k", keychain, "-f", "pkcs12", "-P", password)
	if out := run("find-identity", "-p", "codesigning", keychain); !strings.Contains(out, `"Zapp PEM Signing"`) {
		t.Fatalf("imported identity not found:\n%s", out)
	}
}
