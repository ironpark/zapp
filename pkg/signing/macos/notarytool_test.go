package macos

import (
	"encoding/base64"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The API key is read in the JSON rcodesign writes, and a file missing a part
// says how to make a complete one.
func TestReadAPIKey(t *testing.T) {
	dir := t.TempDir()
	write := func(name, data string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	key, err := readAPIKey(write("key.json", `{"issuer_id":"issuer","key_id":"ABC123","private_key":"-----BEGIN PRIVATE KEY-----\nx\n-----END PRIVATE KEY-----\n"}`))
	if err != nil {
		t.Fatal(err)
	}
	if key.IssuerID != "issuer" || key.KeyID != "ABC123" || !strings.HasPrefix(key.PrivateKey, "-----BEGIN PRIVATE KEY-----") {
		t.Fatalf("key = %+v", key)
	}
	if _, err := readAPIKey(write("partial.json", `{"issuer_id":"issuer"}`)); err == nil || !strings.Contains(err.Error(), "encode-app-store-connect-api-key") {
		t.Fatalf("partial key: %v", err)
	}
	if _, err := readAPIKey(write("bad.json", `AuthKey`)); err == nil {
		t.Fatal("accepted a non-JSON key")
	}
}

// notarytool gets the key as the PEM .p8 whichever way the JSON holds it:
// base64 DER as rcodesign writes it, or PEM as earlier actions did.
func TestAPIKeyP8(t *testing.T) {
	der := []byte{0x30, 0x03, 0x02, 0x01, 0x01}
	want := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	for _, private := range []string{base64.StdEncoding.EncodeToString(der), "MAMC\nAQE=", want} {
		got, err := apiKey{PrivateKey: private}.p8()
		if err != nil || string(got) != want {
			t.Fatalf("p8(%q) = %q, %v", private, got, err)
		}
	}
	if _, err := (apiKey{PrivateKey: "not a key!"}).p8(); err == nil {
		t.Fatal("accepted a key neither PEM nor base64")
	}
}

func TestTimeoutArgs(t *testing.T) {
	for timeout, want := range map[time.Duration]string{0: "", time.Hour: "--timeout 3600s", 1500 * time.Millisecond: "--timeout 2s"} {
		if got := strings.Join(timeoutArgs(timeout), " "); got != want {
			t.Errorf("timeoutArgs(%s) = %q, want %q", timeout, got, want)
		}
	}
}
