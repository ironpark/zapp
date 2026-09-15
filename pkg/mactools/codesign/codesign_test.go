package codesign

import "testing"

func TestBuildArgsDefaults(t *testing.T) {
	options := &Options{IdentityName: "ID", FilePath: "/tmp/App.app", Force: true, Runtime: true, DeepSign: true}
	args := buildArgs(options)

	want := map[string]bool{"--sign": true, "--force": true, "--deep": true, "--options=runtime": true}
	for _, a := range args {
		delete(want, a)
	}
	if len(want) != 0 {
		t.Errorf("buildArgs() is missing %v; got %q", want, args)
	}
	if args[len(args)-1] != "/tmp/App.app" {
		t.Errorf("the file path must come last; got %q", args)
	}
}
