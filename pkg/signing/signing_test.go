package signing

import (
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
	for _, creds := range []Credentials{{P12File: "cert.p12"}, {PEMFile: "cert.pem"}, {APIKeyFile: "key.json"}, {P12Password: "secret"}} {
		if _, err := Select(creds); err == nil {
			t.Fatal("macOS accepted rcodesign credentials")
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
