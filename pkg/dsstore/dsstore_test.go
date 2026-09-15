package dsstore

import (
	"encoding/binary"
	"github.com/ironpark/zapp/internal/thirdparty/text/unicode/norm"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/ironpark/zapp/pkg/dsstore/entry"
)

func TestUnicodeFilenameEncoding(t *testing.T) {
	name := "사용 안내😀.txt"
	encoded := entryBuild(entry.NewIconLocationEntry(name, 100, 200))
	n := int(binary.BigEndian.Uint32(encoded[:4]))
	if n != len(utf16.Encode([]rune(norm.NFD.String(name)))) {
		t.Fatalf("incorrect UTF-16 length: %d", n)
	}
	if got := string(encoded[4+n*2 : 8+n*2]); got != "Iloc" {
		t.Fatalf("misaligned entry type: %q", got)
	}
}
func TestStoreCapacity(t *testing.T) {
	ds := NewDSStore()
	for i := range 100 {
		ds.AddEntry(entry.NewIconLocationEntry(strings.Repeat("a", 50), uint32(i), 0))
	}
	if _, err := ds.EncodeChecked(); err == nil {
		t.Fatal("oversized store accepted")
	}
}
