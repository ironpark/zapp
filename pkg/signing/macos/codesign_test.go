package macos

import (
	"slices"
	"testing"
)

func TestBuildArgsDefaults(t *testing.T) {
	args := codesignArgs("ID", "/tmp/App.app", "")

	want := map[string]bool{"--sign": true, "--force": true, "--deep": true, "--options=runtime": true}
	for _, a := range args {
		delete(want, a)
	}
	if len(want) != 0 {
		t.Errorf("codesignArgs() is missing %v; got %q", want, args)
	}
	if args[len(args)-1] != "/tmp/App.app" {
		t.Errorf("the file path must come last; got %q", args)
	}
	if slices.Contains(args, "--keychain") {
		t.Errorf("no keychain was given, but got %q", args)
	}
}

func TestBuildArgsKeychain(t *testing.T) {
	args := codesignArgs("ID", "/tmp/App.app", "/tmp/signing.keychain-db")
	i := slices.Index(args, "--keychain")
	if i < 0 || i+1 >= len(args) || args[i+1] != "/tmp/signing.keychain-db" {
		t.Fatalf("keychain not passed: %q", args)
	}
	if args[len(args)-1] != "/tmp/App.app" {
		t.Errorf("the file path must come last; got %q", args)
	}
}
