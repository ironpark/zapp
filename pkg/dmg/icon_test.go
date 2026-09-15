package dmg

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// sampleICNS is not a real icon; the resource fork encoder is agnostic to its
// payload, and the tests that need a loadable icon use a real one.
var sampleICNS = []byte("icns\x00\x00\x00\x10TOC \x00\x00\x00\x08")

func TestBuildResourceForkLayout(t *testing.T) {
	fork, err := buildResourceFork("icns", -16455, sampleICNS)
	if err != nil {
		t.Fatalf("buildResourceFork() error: %v", err)
	}

	dataOff := binary.BigEndian.Uint32(fork[0:])
	mapOff := binary.BigEndian.Uint32(fork[4:])
	dataLen := binary.BigEndian.Uint32(fork[8:])
	mapLen := binary.BigEndian.Uint32(fork[12:])

	if dataOff != 256 {
		t.Errorf("data offset = %d, want 256", dataOff)
	}
	if got, want := dataLen, uint32(4+len(sampleICNS)); got != want {
		t.Errorf("data length = %d, want %d", got, want)
	}
	// The three sections must exactly tile the fork, with no gap or overlap.
	if mapOff != dataOff+dataLen {
		t.Errorf("map offset = %d, want %d", mapOff, dataOff+dataLen)
	}
	if got, want := uint32(len(fork)), mapOff+mapLen; got != want {
		t.Errorf("fork length = %d, want %d", got, want)
	}

	// Data section: a big-endian length followed by the payload.
	d := fork[dataOff:]
	if got, want := binary.BigEndian.Uint32(d[0:]), uint32(len(sampleICNS)); got != want {
		t.Errorf("resource data length = %d, want %d", got, want)
	}
	if got := string(d[4 : 4+len(sampleICNS)]); got != string(sampleICNS) {
		t.Errorf("resource payload = %q, want %q", got, sampleICNS)
	}

	// Map: type list and name list offsets, then the single type entry.
	m := fork[mapOff:]
	typeListOff := binary.BigEndian.Uint16(m[24:])
	nameListOff := binary.BigEndian.Uint16(m[26:])
	if typeListOff != 28 {
		t.Errorf("type list offset = %d, want 28", typeListOff)
	}
	if int(nameListOff) != len(m) {
		t.Errorf("name list offset = %d, want %d (empty list at the end)", nameListOff, len(m))
	}

	tl := m[typeListOff:]
	if got := binary.BigEndian.Uint16(tl[0:]); got != 0 {
		t.Errorf("type count field = %d, want 0 (one type, stored minus one)", got)
	}
	if got := string(tl[2:6]); got != "icns" {
		t.Errorf("resource type = %q, want %q", got, "icns")
	}
	if got := binary.BigEndian.Uint16(tl[6:]); got != 0 {
		t.Errorf("resource count field = %d, want 0 (one resource, stored minus one)", got)
	}

	refOff := binary.BigEndian.Uint16(tl[8:])
	r := tl[refOff:]
	if got := int16(binary.BigEndian.Uint16(r[0:])); got != -16455 {
		t.Errorf("resource ID = %d, want -16455", got)
	}
	if got := binary.BigEndian.Uint16(r[2:]); got != 0xFFFF {
		t.Errorf("name offset = %#x, want 0xFFFF (unnamed)", got)
	}
	// 24-bit data offset, relative to the start of the data section.
	if got := uint32(r[5])<<16 | uint32(r[6])<<8 | uint32(r[7]); got != 0 {
		t.Errorf("resource data offset = %d, want 0", got)
	}
}

func TestBuildResourceForkRejectsBadType(t *testing.T) {
	if _, err := buildResourceFork("icn", -16455, sampleICNS); err == nil {
		t.Error("buildResourceFork() accepted a 3-character type")
	}
}

// TestBuildResourceForkAgainstDeRez checks the hand-built fork against the tool
// it replaces: DeRez parses a resource fork and prints its contents, so it only
// succeeds if the layout is genuinely valid.
func TestBuildResourceForkAgainstDeRez(t *testing.T) {
	if _, err := exec.LookPath("DeRez"); err != nil {
		t.Skipf("DeRez not available: %v", err)
	}

	path := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(path, []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := applyCustomIcon(path, sampleICNS); err != nil {
		t.Fatalf("applyCustomIcon() error: %v", err)
	}

	out, err := exec.Command("DeRez", "-only", "icns", path).CombinedOutput()
	if err != nil {
		t.Fatalf("DeRez failed to parse the fork: %v (output: %s)", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "data 'icns' (-16455") {
		t.Errorf("DeRez did not report the icns resource at -16455; output:\n%s", got)
	}
	// The payload should round trip. DeRez prints it as hex grouped into
	// two-byte words, so compare with the spacing removed.
	hex := strings.ToUpper(strings.ReplaceAll(got, " ", ""))
	if !strings.Contains(hex, "69636E73") { // "icns"
		t.Errorf("DeRez output does not contain the payload; output:\n%s", got)
	}
}

func TestApplySetsForkAndFlag(t *testing.T) {
	path := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(path, []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := applyCustomIcon(path, sampleICNS); err != nil {
		t.Fatalf("applyCustomIcon() error: %v", err)
	}

	fork := readAttr(t, path, resourceForkAttr)
	if len(fork) == 0 {
		t.Fatal("resource fork attribute is empty")
	}
	want, err := buildResourceFork("icns", -16455, sampleICNS)
	if err != nil {
		t.Fatal(err)
	}
	if string(fork) != string(want) {
		t.Error("stored resource fork does not match the encoded one")
	}

	info := readAttr(t, path, finderInfoAttr)
	if len(info) != finderInfoSize {
		t.Fatalf("finder info is %d bytes, want %d", len(info), finderInfoSize)
	}
	if flags := binary.BigEndian.Uint16(info[finderFlagsOffset:]); flags&hasCustomIcon == 0 {
		t.Errorf("finder flags = %#04x, want kHasCustomIcon set", flags)
	}

	// The file's own contents must be untouched.
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "payload" {
		t.Errorf("file contents = %q, want %q", body, "payload")
	}
}

func TestApplyRejectsEmptyIcon(t *testing.T) {
	path := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := applyCustomIcon(path, nil); err == nil {
		t.Error("applyCustomIcon() accepted empty icon data")
	}
}

func TestMarkOnDirectoryAndIdempotence(t *testing.T) {
	dir := t.TempDir()
	if err := markCustomIcon(dir); err != nil {
		t.Fatalf("markCustomIcon() on a directory error: %v", err)
	}
	info := readAttr(t, dir, finderInfoAttr)
	if flags := binary.BigEndian.Uint16(info[finderFlagsOffset:]); flags&hasCustomIcon == 0 {
		t.Errorf("finder flags = %#04x, want kHasCustomIcon set", flags)
	}

	// Marking twice must not corrupt anything.
	if err := markCustomIcon(dir); err != nil {
		t.Fatalf("second markCustomIcon() error: %v", err)
	}
	again := readAttr(t, dir, finderInfoAttr)
	if string(again) != string(info) {
		t.Error("second markCustomIcon() changed the finder info")
	}
}

func TestSetCreatorPreservesFlags(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VolumeIcon.icns")
	if err := os.WriteFile(path, sampleICNS, 0644); err != nil {
		t.Fatal(err)
	}
	if err := markCustomIcon(path); err != nil {
		t.Fatal(err)
	}
	if err := setCreatorCode(path, "icnC"); err != nil {
		t.Fatalf("setCreatorCode() error: %v", err)
	}

	info := readAttr(t, path, finderInfoAttr)
	if got := string(info[creatorOffset : creatorOffset+4]); got != "icnC" {
		t.Errorf("creator = %q, want %q", got, "icnC")
	}
	// Writing the creator must not clear a flag set earlier.
	if flags := binary.BigEndian.Uint16(info[finderFlagsOffset:]); flags&hasCustomIcon == 0 {
		t.Errorf("finder flags = %#04x, want kHasCustomIcon still set", flags)
	}
}

func TestSetCreatorRejectsBadCode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := setCreatorCode(path, "abc"); err == nil {
		t.Error("setCreatorCode() accepted a 3-character code")
	}
}

func readAttr(t *testing.T, path, name string) []byte {
	t.Helper()
	size, err := unix.Getxattr(path, name, nil)
	if err != nil {
		t.Fatalf("failed to size %s on %s: %v", name, path, err)
	}
	buf := make([]byte, size)
	n, err := unix.Getxattr(path, name, buf)
	if err != nil {
		t.Fatalf("failed to read %s on %s: %v", name, path, err)
	}
	return buf[:n]
}
