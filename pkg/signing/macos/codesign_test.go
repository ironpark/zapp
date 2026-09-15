package macos

import "testing"

func TestBuildArgsDefaults(t *testing.T) {
	args := codesignArgs("ID", "/tmp/App.app")

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
}
