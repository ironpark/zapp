//go:build !darwin

package signing

import (
	"encoding/base64"
	"errors"
	"os"
	"runtime"
	"testing"

	"github.com/ironpark/zapp/pkg/signing/rcodesign"
)

func TestSelectStaticBackend(t *testing.T) {
	b, err := Select(Credentials{P12File: "cert.p12", P12PasswordFile: "password"})
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
	if got, err := b.Describe(t.Context(), "Demo.app"); err != nil || got != "cert.p12" {
		t.Fatalf("credential lost: %q, %v", got, err)
	}
	// Stapling does not require a private key, so selection must permit it.
	if _, err := Select(Credentials{}); err != nil {
		t.Fatal(err)
	}
}

// A base64 certificate reaches rcodesign as a private file that Close removes.
func TestSelectDecodesBase64Certificate(t *testing.T) {
	if rcodesign.Available() != nil {
		t.Skip("rcodesign is not linked into this build")
	}
	b, err := Select(Credentials{P12Base64: base64.StdEncoding.EncodeToString([]byte("bundle"))})
	if err != nil {
		t.Fatal(err)
	}
	path, err := b.Describe(t.Context(), "Demo.app")
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "bundle" {
		t.Fatalf("decoded certificate = %q, %v", data, err)
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
