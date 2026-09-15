package signing

import (
	"errors"
	"os/exec"
	"runtime"
	"testing"

	"github.com/ironpark/zapp/pkg/mactools/rcodesign"
)

func TestSelectPrefersAppleToolsOnMacOSWithoutACertificateFile(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Apple's tools only exist on macOS")
	}
	b, err := Select(Credentials{Identity: "Developer ID Application"})
	if err != nil {
		t.Fatalf("Select() error: %v", err)
	}
	if _, ok := b.(appleBackend); !ok {
		t.Errorf("Select() chose %s, want Apple's tools", b.Name())
	}
}

// A certificate file is something only rcodesign can read, so naming one is a
// request for it even where Apple's tools are available.
func TestSelectPrefersRcodesignWhenGivenACertificateFile(t *testing.T) {
	if _, err := exec.LookPath(rcodesign.Tool); err != nil {
		t.Skipf("%s not installed: %v", rcodesign.Tool, err)
	}
	for name, creds := range map[string]Credentials{
		"p12":     {Certificate: rcodesign.Credentials{P12File: "/c.p12"}},
		"pem":     {Certificate: rcodesign.Credentials{PEMFile: "/c.pem"}},
		"api key": {APIKeyFile: "/key.json"},
	} {
		t.Run(name, func(t *testing.T) {
			b, err := Select(creds)
			if err != nil {
				t.Fatalf("Select() error: %v", err)
			}
			if _, ok := b.(rcodesignBackend); !ok {
				t.Errorf("Select() chose %s, want rcodesign", b.Name())
			}
		})
	}
}

// Away from macOS there is no keychain, so the absence of a certificate file
// has to be reported as the configuration problem it is.
func TestSelectWithoutCredentials(t *testing.T) {
	_, err := Select(Credentials{})
	if runtime.GOOS == "darwin" {
		if err != nil {
			t.Fatalf("Select() on macOS error: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatal("Select() with no credentials returned no error")
	}
}

// Missing tooling must be reported as itself.
func TestSelectReportsMissingRcodesign(t *testing.T) {
	if _, err := exec.LookPath(rcodesign.Tool); err == nil {
		t.Skip("rcodesign is installed, so this path cannot be reached")
	}
	_, err := Select(Credentials{Certificate: rcodesign.Credentials{P12File: "/c.p12"}})
	if !errors.Is(err, rcodesign.ErrNotInstalled) {
		t.Errorf("Select() error = %v, want it to wrap ErrNotInstalled", err)
	}
}
