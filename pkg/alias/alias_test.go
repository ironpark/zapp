package alias

import (
	"bytes"
	"encoding/binary"
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
	record, err := Create(Target{
		Path:       "/.background/background.png",
		ID:         42,
		ParentID:   17,
		VolumeName: "Test Volume",
	})
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
	if got := int(record[50]); got != len("background.png") {
		t.Errorf("filename length byte = %d, want %d", got, len("background.png"))
	}
	if got := string(record[51 : 51+len("background.png")]); got != "background.png" {
		t.Errorf("filename = %q, want %q", got, "background.png")
	}

	ex := extras(t, record)
	if got := string(ex[0]); got != ".background" {
		t.Errorf("extra 0 (parent name) = %q, want %q", got, ".background")
	}
	if got := binary.BigEndian.Uint32(ex[1]); got != 17 {
		t.Errorf("extra 1 (parent id) = %d, want 17", got)
	}
	if got, want := ex[14], utf16be("background.png"); !bytes.Equal(got[2:], want) {
		t.Errorf("extra 14 (filename UTF-16) = %x, want %x", got[2:], want)
	}
	// Type 18 is the target path relative to the volume root, type 19 the
	// volume mount path.
	if got := string(ex[18]); got != "/.background/background.png" {
		t.Errorf("extra 18 (path in volume) = %q", got)
	}
	if got := string(ex[19]); got != "/Volumes/Test Volume" {
		t.Errorf("extra 19 (mount path) = %q", got)
	}
}

// The counts in extras 14 and 15 are in UTF-16 code units, not bytes, so a name
// outside ASCII must not be reported as longer than it is.
func TestCreateCountsUTF16Units(t *testing.T) {
	record, err := Create(Target{Path: "/배경/그림.png", ID: 1, ParentID: 2, VolumeName: "한글 볼륨"})
	if err != nil {
		t.Fatal(err)
	}
	ex := extras(t, record)
	for _, c := range []struct {
		kind int16
		want string
	}{{14, "그림.png"}, {15, "한글 볼륨"}} {
		units := int(binary.BigEndian.Uint16(ex[c.kind]))
		if want := len(utf16be(c.want)) / 2; units != want {
			t.Errorf("extra %d declares %d units, want %d", c.kind, units, want)
		}
		if got := ex[c.kind][2:]; !bytes.Equal(got, utf16be(c.want)) {
			t.Errorf("extra %d = %x, want %x", c.kind, got, utf16be(c.want))
		}
	}
}

func TestCreateDirectory(t *testing.T) {
	record, err := Create(Target{Path: "/Some.app", ID: 20, ParentID: 2, IsDir: true, VolumeName: "Test Volume"})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if got := binary.BigEndian.Uint16(record[8:]); got != 1 {
		t.Errorf("target type = %d, want 1 (directory)", got)
	}
	// An entry at the root names the volume as its parent.
	if got := string(extras(t, record)[0]); got != "Test Volume" {
		t.Errorf("extra 0 (parent name) = %q, want the volume name", got)
	}
}

func TestCreateRejectsBadTarget(t *testing.T) {
	for name, target := range map[string]Target{
		"no volume":     {Path: "/a.txt"},
		"relative path": {Path: "a.txt", VolumeName: "V"},
		"unclean path":  {Path: "/a/../b.txt", VolumeName: "V"},
		"volume root":   {Path: "/", VolumeName: "V"},
		"empty path":    {VolumeName: "V"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Create(target); err == nil {
				t.Error("expected an error")
			}
		})
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

func TestCreateRecordsTheGivenVolumeName(t *testing.T) {
	const volume = "SyncMaster"
	record, err := Create(Target{Path: "/target.txt", ID: 16, ParentID: 2, VolumeName: volume})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	// The volume name is stored twice: a Pascal string in the base record, and
	// UTF-16 in extra 15.
	if got := int(record[10]); got != len(volume) {
		t.Errorf("volume name length byte = %d, want %d", got, len(volume))
	}
	if got := string(record[11 : 11+len(volume)]); got != volume {
		t.Errorf("volume name = %q, want %q", got, volume)
	}
	if got, want := extras(t, record)[15], utf16be(volume); !bytes.Equal(got[2:], want) {
		t.Errorf("extra 15 (volume name UTF-16) = %x, want %x", got[2:], want)
	}
}
