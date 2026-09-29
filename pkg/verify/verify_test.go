package verify

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// app lays out a bundle whose executable is exe's bytes.
func app(t *testing.T, exe []byte, plist string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Demo.app")
	if err := os.MkdirAll(filepath.Join(path, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "Contents", "MacOS", "Demo"), exe, 0o755); err != nil {
		t.Fatal(err)
	}
	info := `<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleExecutable</key><string>Demo</string><key>CFBundleIdentifier</key><string>dev.zapp.demo</string>` + plist + `</dict></plist>`
	if err := os.WriteFile(filepath.Join(path, "Contents", "Info.plist"), []byte(info), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// thinMachO is an arm64 executable with no load commands but LC_BUILD_VERSION.
func thinMachO(minOS uint32) []byte {
	b := make([]byte, 32+24)
	le := binary.LittleEndian
	le.PutUint32(b, 0xfeedfacf)
	le.PutUint32(b[4:], 0x0100000c)
	le.PutUint32(b[12:], 2)  // MH_EXECUTE
	le.PutUint32(b[16:], 1)  // ncmds
	le.PutUint32(b[20:], 24) // sizeofcmds
	le.PutUint32(b[32:], 0x32)
	le.PutUint32(b[36:], 24)
	le.PutUint32(b[40:], 1) // macOS
	le.PutUint32(b[44:], minOS)
	return b
}

func check(t *testing.T, r Report, name string) Check {
	t.Helper()
	for _, c := range r.Checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no %s check in %+v", name, r.Checks)
	return Check{}
}

func TestUnsignedApp(t *testing.T) {
	r := App(t.Context(), app(t, thinMachO(13<<16), `<key>LSMinimumSystemVersion</key><string>11.0</string>`))
	if c := check(t, r, "signature"); c.Status != Fail || !strings.Contains(c.Detail, "not signed: Contents/MacOS/Demo") {
		t.Errorf("signature: %+v", c)
	}
	if c := check(t, r, "minimum macOS"); c.Status != Warn || !strings.Contains(c.Detail, "needs 13.0 on arm64") {
		t.Errorf("minimum macOS: %+v", c)
	}
	if c := check(t, r, "stapled"); c.Status != Warn {
		t.Errorf("stapled: %+v", c)
	}
	if !r.Failed() {
		t.Error("an unsigned app passed")
	}
}

func TestBrokenBundle(t *testing.T) {
	path := app(t, thinMachO(0), "")
	if err := os.Remove(filepath.Join(path, "Contents", "MacOS", "Demo")); err != nil {
		t.Fatal(err)
	}
	if c := check(t, App(t.Context(), path), "bundle"); c.Status != Fail || !strings.Contains(c.Detail, "missing") {
		t.Errorf("bundle: %+v", c)
	}
	if runtime.GOOS == "windows" {
		return
	}
	path = app(t, thinMachO(0), "")
	if err := os.Chmod(filepath.Join(path, "Contents", "MacOS", "Demo"), 0o644); err != nil {
		t.Fatal(err)
	}
	if c := check(t, App(t.Context(), path), "bundle"); c.Status != Fail || !strings.Contains(c.Detail, "not executable") {
		t.Errorf("bundle: %+v", c)
	}
}

// A system binary is signed by Apple, but not with a Developer ID, and
// without a timestamp or the hardened runtime; an ad hoc signature has no
// certificate at all.
func TestAppleSignatures(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("needs a signed system binary and codesign")
	}
	exe, err := os.ReadFile("/bin/ls")
	if err != nil {
		t.Fatal(err)
	}
	r := App(t.Context(), app(t, exe, ""))
	for name, want := range map[string]string{"Developer ID": "Software Signing", "secure timestamp": "missing on Contents/MacOS/Demo", "hardened runtime": "missing on Contents/MacOS/Demo"} {
		if c := check(t, r, name); c.Status != Fail || !strings.Contains(c.Detail, want) {
			t.Errorf("%s: %+v", name, c)
		}
	}
	if c := check(t, r, "architectures"); !strings.Contains(c.Detail, "arm64") {
		t.Errorf("architectures: %+v", c)
	}

	path := app(t, exe, "")
	if out, err := exec.Command("codesign", "--force", "--sign", "-", path).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if c := check(t, App(t.Context(), path), "signature"); c.Status != Fail || !strings.Contains(c.Detail, "ad hoc") {
		t.Errorf("ad hoc: %+v", c)
	}
}

func TestUnsignedInstallers(t *testing.T) {
	dir := t.TempDir()
	dmg := filepath.Join(dir, "Demo.dmg")
	koly := make([]byte, 1024)
	copy(koly[512:], "koly")
	if err := os.WriteFile(dmg, koly, 0o644); err != nil {
		t.Fatal(err)
	}
	if c := check(t, DMG(t.Context(), dmg), "signature"); c.Status != Fail || !strings.Contains(c.Detail, "not signed") {
		t.Errorf("dmg: %+v", c)
	}
	if err := os.WriteFile(filepath.Join(dir, "x.dmg"), []byte("not a disk image"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !DMG(t.Context(), filepath.Join(dir, "x.dmg")).Failed() {
		t.Error("accepted a non-image")
	}
	if !PKG(t.Context(), filepath.Join(dir, "x.dmg")).Failed() {
		t.Error("accepted a non-package")
	}
	if _, err := Path(t.Context(), filepath.Join(dir, "Demo.exe")); err == nil {
		t.Error("verified an unknown kind of file")
	}
}

func TestHelpers(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{{"13.0", "11.0", true}, {"11", "11.0", false}, {"10.15", "10.9", true}, {"", "11.0", false}, {"12.0.1", "12.0", true}} {
		if got := newer(c.a, c.b); got != c.want {
			t.Errorf("newer(%s, %s) = %v", c.a, c.b, got)
		}
	}
	if !getTaskAllow([]byte("<dict><key>com.apple.security.get-task-allow</key>\n\t<true/></dict>")) || getTaskAllow([]byte("<key>com.apple.security.get-task-allow</key><false/>")) {
		t.Error("getTaskAllow")
	}
	if got := list([]string{"a", "b", "c", "d", "e", "f", "g"}); got != "a, b, c, d, e and 2 more" {
		t.Errorf("list = %q", got)
	}
	// An indefinite-length sequence holding a constructed octet string.
	ber := []byte{0x30, 0x80, 0x24, 0x80, 0x04, 0x01, 'a', 0x04, 0x01, 'b', 0x00, 0x00, 0x02, 0x01, 0x05, 0x00, 0x00}
	der, err := berToDER(ber)
	if err != nil || string(der) != "\x30\x07\x04\x02ab\x02\x01\x05" {
		t.Errorf("berToDER = %x, %v", der, err)
	}
	if _, err := berToDER([]byte{0x30, 0x80, 0x04}); err == nil {
		t.Error("accepted truncated BER")
	}
}
