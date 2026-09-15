package plist

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ironpark/zapp/pkg/appbundle"
	"github.com/ironpark/zapp/pkg/plist"
)

func write(t *testing.T, root map[string]any, binary bool) string {
	t.Helper()
	var data []byte
	var err error
	if binary {
		data, err = plist.MarshalBinary(root)
	} else {
		data, err = plist.MarshalXML(root)
	}
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "Info.plist")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func read(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	root, err := plist.ParseDict(data)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestSetKeepsTheTypeAKeyAlreadyHas(t *testing.T) {
	path := write(t, map[string]any{
		"LSUIElement": true,
		"BuildNumber": int64(42),
		"Version":     "1.0",
	}, false)
	for _, c := range []struct {
		key, literal string
		want         any
	}{
		{"LSUIElement", "false", false},
		{"BuildNumber", "99", int64(99)},
		{"Version", "2.0", "2.0"},
		{"NewFlag", "true", true},
	} {
		if err := setValue(path, c.key, c.literal); err != nil {
			t.Fatal(err)
		}
		if got := read(t, path)[c.key]; got != c.want {
			t.Fatalf("%s: got %#v, want %#v", c.key, got, c.want)
		}
	}
	if err := setValue(path, "LSUIElement", "perhaps"); err == nil {
		t.Fatal("a non-boolean was accepted for a boolean key")
	}
}

func TestSetAndDeleteReachNestedKeys(t *testing.T) {
	path := write(t, map[string]any{
		"NSAppTransportSecurity": map[string]any{"NSAllowsArbitraryLoads": true},
	}, false)
	key := "NSAppTransportSecurity.NSAllowsArbitraryLoads"
	if err := setValue(path, key, "false"); err != nil {
		t.Fatal(err)
	}
	inner := read(t, path)["NSAppTransportSecurity"].(map[string]any)
	if inner["NSAllowsArbitraryLoads"] != false {
		t.Fatalf("nested set: %#v", inner)
	}
	if err := deleteValue(path, key); err != nil {
		t.Fatal(err)
	}
	inner = read(t, path)["NSAppTransportSecurity"].(map[string]any)
	if _, ok := inner["NSAllowsArbitraryLoads"]; ok {
		t.Fatalf("nested delete left %#v", inner)
	}
	if err := deleteValue(path, "NotThere"); err == nil {
		t.Fatal("removing a missing key succeeded")
	}
}

// Rewriting a binary Info.plist as XML would invalidate the signature of the
// bundle holding it, and the file's permissions are not ours to widen.
func TestEditPreservesFormatAndPermissions(t *testing.T) {
	for _, binary := range []bool{true, false} {
		path := write(t, map[string]any{"CFBundleVersion": "1.0"}, binary)
		if err := setValue(path, "CFBundleVersion", "2.0"); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := len(data) > 0 && data[0] == 'b'; got != binary {
			t.Fatalf("binary=%v was rewritten as binary=%v", binary, got)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("permissions became %v", info.Mode().Perm())
		}
	}
}

func TestRaise(t *testing.T) {
	for _, c := range []struct {
		version   string
		component int
		want      string
		wantErr   bool
	}{
		{"1.4.2", -1, "1.4.3", false},
		{"1.4.2", 0, "2.0.0", false},
		{"1.4.2", 1, "1.5.0", false},
		{"1.4.2", 2, "1.4.3", false},
		{"42", -1, "43", false},
		{"1.4.2", 3, "", true},
		{"1.0.beta", -1, "", true},
		{"", -1, "", true},
	} {
		got, err := raise(c.version, c.component)
		if c.wantErr {
			if err == nil {
				t.Fatalf("raise(%q,%d) = %q, want error", c.version, c.component, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Fatalf("raise(%q,%d) = %q,%v want %q", c.version, c.component, got, err, c.want)
		}
	}
}

func TestBumpKeepsTheKeysType(t *testing.T) {
	path := write(t, map[string]any{
		"CFBundleVersion": "1.4.2",
		"BuildNumber":     int64(42),
		"Flag":            true,
	}, false)

	before, after, err := bumpValue(path, "CFBundleVersion", 1)
	if err != nil || before != "1.4.2" || after != "1.5.0" {
		t.Fatalf("minor bump: %q -> %q (%v)", before, after, err)
	}
	if got := read(t, path)["CFBundleVersion"]; got != "1.5.0" {
		t.Fatalf("version became %#v", got)
	}

	if _, _, err := bumpValue(path, "BuildNumber", -1); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path)["BuildNumber"]; got != int64(43) {
		t.Fatalf("integer build number became %#v", got)
	}

	if _, _, err := bumpValue(path, "Flag", -1); err == nil {
		t.Fatal("bumped a boolean")
	}
	if _, _, err := bumpValue(path, "Missing", -1); err == nil {
		t.Fatal("bumped a missing key")
	}
}

func TestFindPlistPathAcceptsBundles(t *testing.T) {
	dir := t.TempDir()
	bundle := filepath.Join(dir, "My.app")
	if err := os.MkdirAll(filepath.Join(bundle, "Contents"), 0755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(bundle, "Contents", "Info.plist")
	if err := os.WriteFile(want, []byte("<plist><dict/></plist>"), 0644); err != nil {
		t.Fatal(err)
	}
	// A shell completing the bundle name leaves the trailing separator.
	for _, in := range []string{bundle, bundle + "/"} {
		got, err := appbundle.FindPlistPath(in)
		if err != nil || got != want {
			t.Fatalf("FindPlistPath(%q) = %q,%v want %q", in, got, err, want)
		}
	}
	if _, err := appbundle.FindPlistPath(dir); err == nil {
		t.Fatal("a plain directory was accepted")
	}
}
