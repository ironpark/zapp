package codesign

import "testing"

func TestIsStub(t *testing.T) {
	// Verbatim from Darling's codesign, which exits zero after printing it.
	const darling = "codesign DID NOT ACTUALLY VERIFY THE SIGNATURE OF ANY CODE THIS IS JUST A STUB\n"
	if !isStub(darling) {
		t.Error("isStub() did not recognise Darling's notice")
	}
	// Real codesign is silent on success, and verbose output must not trip it.
	for _, out := range []string{
		"",
		"/tmp/App.app: replacing existing signature\n",
		"/tmp/App.app: signed app bundle with Mach-O thin (arm64) [dev.zapp.test]\n",
	} {
		if isStub(out) {
			t.Errorf("isStub(%q) = true, want false", out)
		}
	}
}

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
