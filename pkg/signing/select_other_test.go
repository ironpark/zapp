//go:build !darwin

package signing

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ironpark/zapp/pkg/signing/rcodesign"
)

func TestSelectStaticBackend(t *testing.T) {
	key := ecKey(t)
	file := writePEM(t, pkcs8PEM(t, key), certPEM(testCert(t, "Zapp Test Signing", key, nil, nil)))
	b, err := Select(Credentials{PEMFile: file})
	if rcodesign.Available() != nil {
		if !errors.Is(err, rcodesign.ErrUnavailable) {
			t.Fatalf("got %v", err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = Close(b) }()
	if got, err := b.Describe(t.Context(), "Demo.app"); err != nil || got != file {
		t.Fatalf("credential lost: %q, %v", got, err)
	}
	// Stapling does not require a private key, so selection must permit it.
	b, err = Select(Credentials{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := b.(*rcodesign.Backend); !ok {
		t.Fatalf("unexpected backend %T", b)
	}
}

// A PKCS#12 certificate, from a file or base64, reaches rcodesign unpacked as
// a private PEM file that Close removes, while Describe still names what the
// user supplied.
func TestSelectUnpacksP12(t *testing.T) {
	if rcodesign.Available() != nil {
		t.Skip("rcodesign is not linked into this build")
	}
	p12 := testP12(t, "secret")
	file := filepath.Join(t.TempDir(), "cert.p12")
	if err := os.WriteFile(file, p12, 0o600); err != nil {
		t.Fatal(err)
	}
	for want, creds := range map[string]Credentials{
		file:                              {P12File: file, P12Password: "secret"},
		"PKCS#12 certificate from base64": {P12Base64: base64.StdEncoding.EncodeToString(p12), P12Password: "secret"},
	} {
		b, err := Select(creds)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := b.Describe(t.Context(), "Demo.app"); err != nil || got != want {
			t.Fatalf("Describe = %q, %v; want %q", got, err, want)
		}
		path, err := b.(*decodedCertificate).Backend.Describe(t.Context(), "Demo.app")
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(data), "PRIVATE KEY") {
			t.Fatalf("unpacked certificate = %q, %v", data, err)
		}
		if info, err := os.Stat(path); err == nil && runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
			t.Fatalf("certificate file is readable by others: %v", info.Mode())
		}
		if err := Close(b); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("certificate file left behind: %v", err)
		}
		if err := Close(b); err != nil {
			t.Fatalf("second Close: %v", err)
		}
	}
	if _, err := Select(Credentials{P12File: file, P12Password: "wrong"}); err == nil || !strings.Contains(err.Error(), "incorrect PKCS#12 password") {
		t.Fatalf("wrong password: %v", err)
	}
	if _, err := Select(Credentials{P12File: file, PEMFile: "cert.pem"}); err == nil {
		t.Fatal("accepted two certificates")
	}
}

// A key JSON holding the PEM, which rcodesign would reject as an invalid
// unified key, is refused up front with the format it needs.
func TestSelectRejectsPEMAPIKey(t *testing.T) {
	dir := t.TempDir()
	write := func(name, private string) string {
		path := filepath.Join(dir, name)
		data := `{"issuer_id":"issuer","key_id":"ABC123","private_key":"` + private + `"}`
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	pemKey := write("pem.json", `-----BEGIN PRIVATE KEY-----\nMAMCAQE=\n-----END PRIVATE KEY-----\n`)
	if err := checkAPIKey(pemKey); err == nil || !strings.Contains(err.Error(), "base64 DER") {
		t.Fatalf("PEM key: %v", err)
	}
	if err := checkAPIKey(write("der.json", "MAMCAQE=")); err != nil {
		t.Fatal(err)
	}
	if err := checkAPIKey(""); err != nil {
		t.Fatal(err)
	}
}
