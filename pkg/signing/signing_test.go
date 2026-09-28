package signing

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSelectMacOS(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS selection")
	}
	b, err := Select(Credentials{Identity: "Developer ID Application"})
	if err != nil {
		t.Fatal(err)
	}
	if b.Name() != "Apple codesign" {
		t.Fatalf("unexpected backend %s", b.Name())
	}
	// The rcodesign API key JSON notarizes on macOS too, through notarytool.
	if _, err := Select(Credentials{APIKeyFile: "key.json"}); err != nil {
		t.Fatal(err)
	}
}

// macOS takes a PKCS#12 certificate from a file or base64, in any encryption,
// imported into a temporary keychain when it is first used; selecting does not
// touch the keychain.
func TestSelectMacOSAcceptsP12(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS selection")
	}
	p12 := testP12(t, "secret")
	file := filepath.Join(t.TempDir(), "cert.p12")
	if err := os.WriteFile(file, p12, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, creds := range []Credentials{{P12File: file, P12Password: "secret"}, {P12Base64: base64.StdEncoding.EncodeToString(p12), P12Password: "secret"}} {
		b, err := Select(creds)
		if err != nil {
			t.Fatalf("%+v: %v", creds, err)
		}
		if err := Close(b); err != nil {
			t.Fatal(err)
		}
	}
}

// A PEM bundle is read when selected, so a bad one fails here rather than at
// the keychain import, and it cannot be combined with a PKCS#12 certificate.
func TestSelectMacOSAcceptsPEM(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS selection")
	}
	key := ecKey(t)
	file := writePEM(t, pkcs8PEM(t, key), certPEM(testCert(t, "Zapp Test Signing", key, nil, nil)))
	b, err := Select(Credentials{PEMFile: file})
	if err != nil {
		t.Fatal(err)
	}
	if err := Close(b); err != nil {
		t.Fatal(err)
	}
	for name, creds := range map[string]Credentials{
		"with a PKCS#12":   {PEMFile: file, P12Base64: "cDEy"},
		"certificate only": {PEMFile: writePEM(t, certPEM(testCert(t, "Zapp Test Signing", key, nil, nil)))},
		"with a password":  {PEMFile: file, P12Password: "x"},
	} {
		if _, err := Select(creds); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
func TestSelectRejectsAppleCredentialsAwayFromMacOS(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("non-macOS selection")
	}
	for _, creds := range []Credentials{{Identity: "Developer ID"}, {Profile: "notary"}, {AppleID: "user@example.com"}} {
		if _, err := Select(creds); err == nil {
			t.Fatal("accepted macOS credentials")
		}
	}
}

func TestP12Options(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "cert.p12")
	passwordFile := filepath.Join(dir, "password.txt")
	if err := os.WriteFile(file, []byte("bundle"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(passwordFile, []byte("from-file\r\nignored\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for name, c := range map[string]Credentials{
		"both forms":         {P12File: file, P12Base64: "YnVuZGxl"},
		"password alone":     {P12Password: "x"},
		"password file only": {P12PasswordFile: passwordFile},
	} {
		if err := c.checkP12(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	// Wrapped base64, as `openssl base64` writes it, decodes to the same bytes.
	wrapped := Credentials{P12Base64: "YnVu\nZGxl\n"}
	if data, err := wrapped.p12(); err != nil || string(data) != "bundle" {
		t.Fatalf("wrapped base64 = %q, %v", data, err)
	}
	if _, err := (Credentials{P12Base64: "not base64!"}).p12(); err == nil {
		t.Fatal("accepted invalid base64")
	}
	if data, err := (Credentials{P12File: file}).p12(); err != nil || string(data) != "bundle" {
		t.Fatalf("file = %q, %v", data, err)
	}

	// A password file wins over a literal password, and only its first line counts.
	c := Credentials{P12File: file, P12Password: "literal", P12PasswordFile: passwordFile}
	if pw, err := c.p12Password(); err != nil || pw != "from-file" {
		t.Fatalf("password = %q, %v", pw, err)
	}
	if pw, _ := (Credentials{P12Password: "literal"}).p12Password(); pw != "literal" {
		t.Fatalf("literal password = %q", pw)
	}
}
