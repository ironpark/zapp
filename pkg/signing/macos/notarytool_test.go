package macos

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
