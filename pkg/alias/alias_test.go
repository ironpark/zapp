package alias

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// extras walks the extra records that follow the 150-byte base record and
// returns them keyed by type.
func extras(t *testing.T, record []byte) map[int16][]byte {
	t.Helper()
	if len(record) < 154 {
		t.Fatalf("record too short: %d bytes", len(record))
	}
	found := map[int16][]byte{}
	for pos := 150; pos+4 <= len(record); {
		typ := int16(binary.BigEndian.Uint16(record[pos:]))
		length := int(binary.BigEndian.Uint16(record[pos+2:]))
		if typ == -1 {
			break
		}
		if pos+4+length > len(record) {
			t.Fatalf("extra type %d claims %d bytes past end of record", typ, length)
		}
		found[typ] = record[pos+4 : pos+4+length]
		pos += 4 + length
		if length%2 != 0 {
			pos++
		}
	}
	return found
}

func TestCreateFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	record, err := Create(target)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// The header declares its own total length; a mismatch means Finder reads
	// past or stops short of the record.
	if got := int(binary.BigEndian.Uint16(record[4:])); got != len(record) {
		t.Errorf("declared length = %d, actual = %d", got, len(record))
	}
	if got := binary.BigEndian.Uint16(record[6:]); got != 2 {
		t.Errorf("version = %d, want 2", got)
	}
	// Type 0 = file, 1 = directory.
	if got := binary.BigEndian.Uint16(record[8:]); got != 0 {
		t.Errorf("target type = %d, want 0 (file)", got)
	}
	if got := int(record[50]); got != len("target.txt") {
		t.Errorf("filename length byte = %d, want %d", got, len("target.txt"))
	}
	if got := string(record[51 : 51+len("target.txt")]); got != "target.txt" {
		t.Errorf("filename = %q, want %q", got, "target.txt")
	}

	ex := extras(t, record)
	if got := string(ex[0]); got != filepath.Base(dir) {
		t.Errorf("extra 0 (parent name) = %q, want %q", got, filepath.Base(dir))
	}
	if got, want := ex[14], utf16be("target.txt"); !bytes.Equal(got[2:], want) {
		t.Errorf("extra 14 (filename UTF-16) = %x, want %x", got[2:], want)
	}
	// Type 18 is the target path relative to the volume root, type 19 the
	// volume mount path. Joined they must reproduce the target.
	if got := filepath.Join(string(ex[19]), string(ex[18])); got != target {
		t.Errorf("extra 19+18 = %q, want %q", got, target)
	}
}

func TestCreateDirectory(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "Some.app")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}

	record, err := Create(target)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if got := binary.BigEndian.Uint16(record[8:]); got != 1 {
		t.Errorf("target type = %d, want 1 (directory)", got)
	}
}

func TestCreateMissingTarget(t *testing.T) {
	if _, err := Create(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("Create() on a missing path returned no error")
	}
}

func TestEncodeRejectsInvalidInfo(t *testing.T) {
	valid := func() Info {
		info := Info{Version: 2}
		info.Target.Type = "file"
		info.Target.Filename = "a.txt"
		info.Volume.Name = "Macintosh HD"
		info.Volume.Signature = "H+"
		info.Volume.Type = "local"
		return info
	}
	if _, err := Encode(valid()); err != nil {
		t.Fatalf("Encode(valid) error: %v", err)
	}

	for name, mutate := range map[string]func(*Info){
		"bad version":     func(i *Info) { i.Version = 3 },
		"bad target type": func(i *Info) { i.Target.Type = "symlink" },
		"bad volume sig":  func(i *Info) { i.Volume.Signature = "ZZ" },
		"bad volume type": func(i *Info) { i.Volume.Type = "tape" },
		"volume name too long": func(i *Info) {
			i.Volume.Name = "0123456789012345678901234567890"
		},
		"filename too long": func(i *Info) {
			i.Target.Filename = string(make([]byte, 64))
		},
		"extra length mismatch": func(i *Info) {
			i.Extra = []Extra{{Type: 0, Length: 9, Data: []byte("abc")}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			info := valid()
			mutate(&info)
			if _, err := Encode(info); err == nil {
				t.Errorf("Encode(%s) returned no error", name)
			}
		})
	}
}

func TestGetVolumeName(t *testing.T) {
	name, err := GetVolumeName("/")
	if err != nil {
		t.Fatalf("GetVolumeName(/) error: %v", err)
	}
	if name == "" {
		t.Error("GetVolumeName(/) returned an empty name")
	}
}
