package dmg

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"

	"github.com/ironpark/zapp/internal/thirdparty/text/unicode/norm"
)

func TestRenamedContents(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(source, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	c := Config{Title: "Layout", FileName: filepath.Join(t.TempDir(), "layout.dmg"), WindowWidth: 640, WindowHeight: 480, ContentsIconSize: 96, LabelSize: 14, Contents: []Item{{Type: File, Path: source, Name: "사용 안내.txt", X: 120, Y: 100}, {Type: Link, Path: "/Applications", Name: "Install", X: 400, Y: 100}}}
	volume, err := c.buildVolume()
	if err != nil {
		t.Fatal(err)
	}
	if volume.Root.Child("사용 안내.txt") == nil || volume.Root.Child("source.txt") != nil || volume.Root.Child("Install").LinkTarget != "/Applications" {
		t.Fatal("incorrect renamed volume entries")
	}
	data, err := c.buildStore(volume)
	if err != nil {
		t.Fatal(err)
	}
	var name bytes.Buffer
	for _, u := range utf16.Encode([]rune(norm.NFD.String("사용 안내.txt"))) {
		_ = binary.Write(&name, binary.BigEndian, u)
	}
	if !bytes.Contains(data, append(name.Bytes(), []byte("Iloc")...)) {
		t.Fatal("layout does not reference renamed file")
	}
	if err := CreateDMG(t.Context(), c); err != nil {
		t.Fatal(err)
	}
	point := mount(t, c.FileName)
	got, err := os.ReadFile(filepath.Join(point, "사용 안내.txt"))
	if err != nil || string(got) != "hello" {
		t.Fatalf("mounted file: %q, %v", got, err)
	}
	target, err := os.Readlink(filepath.Join(point, "Install"))
	if err != nil || target != "/Applications" {
		t.Fatalf("mounted link: %q, %v", target, err)
	}
}
