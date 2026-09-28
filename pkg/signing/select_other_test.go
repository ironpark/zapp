//go:build !darwin

package signing

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ironpark/zapp/pkg/signing/rcodesign"
	"software.sslmate.com/src/go-pkcs12"
)

func TestSelectStaticBackend(t *testing.T) {
	b, err := Select(Credentials{PEMFile: "cert.pem"})
	if rcodesign.Available() != nil {
		if !errors.Is(err, rcodesign.ErrUnavailable) {
			t.Fatalf("got %v", err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := b.(*rcodesign.Backend); !ok {
		t.Fatalf("unexpected backend %T", b)
	}
	if got, err := b.Describe(t.Context(), "Demo.app"); err != nil || got != "cert.pem" {
		t.Fatalf("credential lost: %q, %v", got, err)
	}
	// Stapling does not require a private key, so selection must permit it.
	if _, err := Select(Credentials{}); err != nil {
		t.Fatal(err)
	}
}

// testP12 is a self-signed certificate and key as PKCS#12 in the modern
// encryption rcodesign cannot read itself.
func testP12(t *testing.T, password string) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	p12, err := pkcs12.Modern2023.Encode(key, testCert(t, "Zapp Test Signing", key, nil, nil), nil, password)
	if err != nil {
		t.Fatal(err)
	}
	return p12
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
