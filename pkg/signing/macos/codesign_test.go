package macos

import (
	"slices"
	"testing"
)

func TestBuildArgsDefaults(t *testing.T) {
	args := codesignArgs("ID", "", "", "/tmp/App.app")

	want := map[string]bool{"--sign": true, "--force": true, "--timestamp": true, "--options=runtime": true, "--preserve-metadata=entitlements": true}
	for _, a := range args {
		delete(want, a)
	}
	if len(want) != 0 {
		t.Errorf("codesignArgs() is missing %v; got %q", want, args)
	}
	if args[len(args)-1] != "/tmp/App.app" {
		t.Errorf("the file path must come last; got %q", args)
	}
	for _, flag := range []string{"--keychain", "--deep", "--entitlements"} {
		if slices.Contains(args, flag) {
			t.Errorf("unexpected %s in %q", flag, args)
		}
	}
}

func TestBuildArgsKeychain(t *testing.T) {
	args := codesignArgs("ID", "/tmp/signing.keychain-db", "", "/tmp/App.app")
	i := slices.Index(args, "--keychain")
	if i < 0 || i+1 >= len(args) || args[i+1] != "/tmp/signing.keychain-db" {
		t.Fatalf("keychain not passed: %q", args)
	}
	if args[len(args)-1] != "/tmp/App.app" {
		t.Errorf("the file path must come last; got %q", args)
	}
}

func TestBuildArgsEntitlements(t *testing.T) {
	args := codesignArgs("ID", "", "/tmp/app.entitlements", "/tmp/App.app")
	i := slices.Index(args, "--entitlements")
	if i < 0 || i+1 >= len(args) || args[i+1] != "/tmp/app.entitlements" {
		t.Fatalf("entitlements not passed: %q", args)
	}
	if slices.Contains(args, "--preserve-metadata=entitlements") {
		t.Errorf("an entitlements file replaces the app's own; got %q", args)
	}
}

func TestBuildArgsPaths(t *testing.T) {
	args := codesignArgs("ID", "", "", "/a", "/b")
	if !slices.Equal(args[len(args)-2:], []string{"/a", "/b"}) {
		t.Errorf("every path must be passed, last; got %q", args)
	}
}
