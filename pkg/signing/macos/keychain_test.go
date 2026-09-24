package macos

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestParseKeychainList(t *testing.T) {
	out := `    "/Users/me/Library/Keychains/login.keychain-db"
    "/Library/Keychains/System.keychain"
`
	got := parseKeychainList(out)
	want := []string{"/Users/me/Library/Keychains/login.keychain-db", "/Library/Keychains/System.keychain"}
	if !slices.Equal(got, want) {
		t.Fatalf("parseKeychainList = %q, want %q", got, want)
	}
	if got := parseKeychainList(""); len(got) != 0 {
		t.Fatalf("empty output gave %q", got)
	}
}

// A failed security command must not echo its arguments: they carry the
// PKCS#12 and keychain passwords.
func TestSecurityErrorHidesArguments(t *testing.T) {
	const secret = "hunter2-secret"
	_, err := security(context.Background(), "no-such-subcommand", "-P", secret)
	if err == nil {
		t.Skip("security accepted the subcommand")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaks an argument: %v", err)
	}
	if !strings.Contains(err.Error(), "no-such-subcommand") {
		t.Fatalf("error does not name the subcommand: %v", err)
	}
}
