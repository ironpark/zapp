package macexec

import (
	"reflect"
	"strings"
	"testing"
)

func TestHostPath(t *testing.T) {
	for name, tc := range map[string]struct{ in, want string }{
		"absolute":        {"/home/user/App.app", "/Volumes/SystemRoot/home/user/App.app"},
		"root":            {"/", "/Volumes/SystemRoot"},
		"relative":        {"build/App.dmg", "build/App.dmg"},
		"bare name":       {"App.dmg", "App.dmg"},
		"already mapped":  {"/Volumes/SystemRoot/home/user/x", "/Volumes/SystemRoot/home/user/x"},
		"the mount point": {"/Volumes/SystemRoot", "/Volumes/SystemRoot"},
		// A path that merely starts with the same characters is a different
		// directory and must still be translated.
		"lookalike": {"/Volumes/SystemRootish/x", "/Volumes/SystemRoot/Volumes/SystemRootish/x"},
		// Darwin volumes inside the container are not host paths, but zapp only
		// ever passes host paths, so translating them is the right default.
		"darwin volume": {"/Volumes/MyDMG", "/Volumes/SystemRoot/Volumes/MyDMG"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := hostPath(tc.in); got != tc.want {
				t.Errorf("hostPath(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestTranslateArg(t *testing.T) {
	for name, tc := range map[string]struct{ in, want string }{
		"bare path":        {"/home/u/App.app", "/Volumes/SystemRoot/home/u/App.app"},
		"flag with path":   {"--entitlements=/home/u/e.plist", "--entitlements=/Volumes/SystemRoot/home/u/e.plist"},
		"short flag":       {"-o=/home/u/out.dmg", "-o=/Volumes/SystemRoot/home/u/out.dmg"},
		"plain flag":       {"--force", "--force"},
		"flag with value":  {"--options=runtime", "--options=runtime"},
		"relative":         {"out.dmg", "out.dmg"},
		"separate operand": {"-volname", "-volname"},
		// An identity is not a path even though it contains punctuation.
		"identity": {"Developer ID Application: Some One (TEAMID)", "Developer ID Application: Some One (TEAMID)"},
		// A volume name could start with a slash-free string; leave it alone.
		"volume name": {"SyncMaster", "SyncMaster"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := translateArg(tc.in); got != tc.want {
				t.Errorf("translateArg(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestShellQuote(t *testing.T) {
	for name, tc := range map[string]struct{ in, want string }{
		"plain":         {"hdiutil", `'hdiutil'`},
		"spaces":        {"/home/u/My App.app", `'/home/u/My App.app'`},
		"single quote":  {"it's", `'it'\''s'`},
		"double quote":  {`say "hi"`, `'say "hi"'`},
		"dollar":        {"$HOME", `'$HOME'`},
		"backtick":      {"`id`", "'`id`'"},
		"semicolon":     {"a; rm -rf /", `'a; rm -rf /'`},
		"empty":         {"", `''`},
		"newline":       {"a\nb", "'a\nb'"},
		"already quote": {"'", `''\'''`},
	} {
		t.Run(name, func(t *testing.T) {
			if got := shellQuote(tc.in); got != tc.want {
				t.Errorf("shellQuote(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestDarlingArgv(t *testing.T) {
	got := darlingArgv("/home/u/project", "hdiutil",
		[]string{"create", "-volname", "SyncMaster", "-srcfolder", "/tmp/src", "-ov", "out.dmg"})

	if len(got) != 5 {
		t.Fatalf("argv has %d elements, want 5: %q", len(got), got)
	}
	if want := []string{darlingBin, "shell", "/bin/sh", "-c"}; !reflect.DeepEqual(got[:4], want) {
		t.Errorf("argv prefix = %q, want %q", got[:4], want)
	}

	script := got[4]
	// The working directory has to be translated too, or relative arguments
	// resolve against the container's home directory.
	if !strings.HasPrefix(script, `cd '/Volumes/SystemRoot/home/u/project' && exec `) {
		t.Errorf("script does not cd into the translated working directory: %s", script)
	}
	for _, want := range []string{
		`'hdiutil'`, `'create'`, `'-volname'`, `'SyncMaster'`,
		`'-srcfolder' '/Volumes/SystemRoot/tmp/src'`, `'-ov'`, `'out.dmg'`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("script is missing %s; got: %s", want, script)
		}
	}
	// The volume name must not have been mistaken for a path.
	if strings.Contains(script, "SystemRoot/SyncMaster") {
		t.Errorf("volume name was translated as a path: %s", script)
	}
}

// Arguments reach the shell as data, never as syntax.
func TestDarlingArgvQuotesHostileArguments(t *testing.T) {
	script := darlingArgv("/tmp", "codesign", []string{"--sign", "x'; touch /tmp/pwned; #", "/tmp/My App.app"})[4]

	if strings.Contains(script, "; touch /tmp/pwned") && !strings.Contains(script, `'\''`) {
		t.Errorf("argument escaped its quoting: %s", script)
	}
	if !strings.Contains(script, `'/Volumes/SystemRoot/tmp/My App.app'`) {
		t.Errorf("a path with a space was not quoted as one word: %s", script)
	}
}
