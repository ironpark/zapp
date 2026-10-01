package comp

import (
	"io/fs"
	"strings"
	"testing"
)

func TestEmbeddedIconsExist(t *testing.T) {
	for _, name := range []Icon{IconUndo, IconRedo} {
		data, err := fs.ReadFile(IconFiles(), name.File())
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "<svg") {
			t.Fatalf("%s is not an SVG", name)
		}
	}
}
