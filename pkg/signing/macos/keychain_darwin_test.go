//go:build darwin

package macos

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// testP12 makes a throwaway self-signed code signing certificate as PKCS#12.
// LibreSSL's /usr/bin/openssl writes the legacy encryption `security import`
// reads; OpenSSL 3 would need -legacy.
func testP12(t *testing.T, password string) []byte {
	t.Helper()
	dir := t.TempDir()
	conf := filepath.Join(dir, "openssl.cnf")
	if err := os.WriteFile(conf, []byte(`[req]
distinguished_name = dn
x509_extensions = ext
prompt = no
[dn]
CN = Zapp Test Signing
[ext]
basicConstraints = critical, CA:false
keyUsage = critical, digitalSignature
extendedKeyUsage = critical, codeSigning
`), 0o600); err != nil {
		t.Fatal(err)
	}
	key, cert, p12 := filepath.Join(dir, "key.pem"), filepath.Join(dir, "cert.pem"), filepath.Join(dir, "cert.p12")
	for _, args := range [][]string{
		{"req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", key, "-out", cert, "-days", "1", "-config", conf},
		{"pkcs12", "-export", "-inkey", key, "-in", cert, "-out", p12, "-passout", "pass:" + password, "-name", "Zapp Test Signing"},
	} {
		if out, err := exec.Command("/usr/bin/openssl", args...).CombinedOutput(); err != nil {
			t.Fatalf("openssl %s: %v\n%s", args[0], err, out)
		}
	}
	data, err := os.ReadFile(p12)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func requireKeychainTest(t *testing.T) {
	t.Helper()
	if os.Getenv("ZAPP_KEYCHAIN_TEST") == "" {
		t.Skip("changes the user keychain search list while it runs; set ZAPP_KEYCHAIN_TEST=1 to run")
	}
}

// The real import: the certificate becomes usable by codesign, and closing
// leaves the search list as it was and the keychain deleted.
func TestP12ImportSignAndCleanup(t *testing.T) {
	requireKeychainTest(t)
	ctx := context.Background()
	const password = "zapp-test-password"
	p12 := testP12(t, password)
	before, err := searchList(ctx)
	if err != nil {
		t.Fatal(err)
	}

	k, err := importP12(ctx, p12, password)
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	t.Cleanup(func() {
		if !closed {
			_ = k.close(ctx)
		}
	})
	during, err := searchList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(during, func(p string) bool { return samePath(p, k.path) }) {
		t.Fatalf("keychain not on the search list: %q", during)
	}

	// A self-signed certificate is not a valid identity, so look it up
	// without -v to find its fingerprint.
	out, err := security(ctx, "find-identity", "-p", "codesigning", k.path)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`([A-F0-9]{40}) "Zapp Test Signing"`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("imported identity not found:\n%s", out)
	}
	target := filepath.Join(t.TempDir(), "true")
	data, err := os.ReadFile("/usr/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runCodesign(ctx, m[1], target, k.path); err != nil {
		t.Fatalf("codesign with the imported key: %v", err)
	}
	info, err := exec.Command("codesign", "-dvv", target).CombinedOutput()
	if err != nil || !strings.Contains(string(info), "Authority=Zapp Test Signing") {
		t.Fatalf("signature not made by the imported certificate: %v\n%s", err, info)
	}

	if err := k.close(ctx); err != nil {
		t.Fatal(err)
	}
	closed = true
	after, err := searchList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(before, after) {
		t.Fatalf("search list changed:\nbefore %q\nafter  %q", before, after)
	}
	if _, err := os.Stat(k.dir); !os.IsNotExist(err) {
		t.Fatalf("keychain directory left behind: %v", err)
	}
}

// A wrong password fails the import without leaving a keychain behind or
// printing the password.
func TestP12ImportWrongPassword(t *testing.T) {
	requireKeychainTest(t)
	ctx := context.Background()
	p12 := testP12(t, "right-password")
	before, err := searchList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	const wrong = "wrong-password-value"
	if _, err := importP12(ctx, p12, wrong); err == nil {
		t.Fatal("imported with the wrong password")
	} else if strings.Contains(err.Error(), wrong) {
		t.Fatalf("error leaks the password: %v", err)
	}
	after, err := searchList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(before, after) {
		t.Fatalf("search list changed:\nbefore %q\nafter  %q", before, after)
	}
}

// Through the backend: signing imports on first use, reports that a
// self-signed certificate is no valid Developer ID identity, and Close cleans
// up.
func TestBackendP12Lifecycle(t *testing.T) {
	requireKeychainTest(t)
	ctx := context.Background()
	const password = "zapp-test-password"
	before, err := searchList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b := New(Options{P12: testP12(t, password), P12Password: password})
	err = b.Sign(ctx, filepath.Join(t.TempDir(), "Demo.app"))
	if err == nil || !strings.Contains(err.Error(), "no valid signing identity") {
		t.Fatalf("expected an invalid-identity error, got %v", err)
	}
	if b.keychain == nil {
		t.Fatal("certificate was not imported")
	}
	dir := b.keychain.dir
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	after, err := searchList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(before, after) {
		t.Fatalf("search list changed:\nbefore %q\nafter  %q", before, after)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("keychain directory left behind: %v", err)
	}
}
