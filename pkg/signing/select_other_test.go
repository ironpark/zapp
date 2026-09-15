//go:build !darwin

package signing

import (
	"errors"
	"github.com/ironpark/zapp/pkg/signing/rcodesign"
	"testing"
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
