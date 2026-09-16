package gui

import (
	"bytes"
	"image/png"
	"testing"
)

func TestFileIconType(t *testing.T) {
	for _, tc := range []struct {
		path      string
		directory bool
		want      string
	}{
		{"Demo.APP", true, "app"}, {"notes.pdf", true, "folder"},
		{"/Applications", true, "applications"}, {"README.md", false, "text"},
		{"report.PDF", false, "pdf"}, {"photo.HEIC", false, "image"},
		{"sound.flac", false, "audio"}, {"movie.mov", false, "video"},
		{"backup.tar.gz", false, "archive"}, {"release.dmg", false, "disk"},
		{"install.pkg", false, "package"}, {"font.otf", false, "font"},
		{"setup.command", false, "script"}, {"main.go", false, "source"},
		{"tool.bin", false, "executable"}, {"unknown.xyz", false, "file"},
	} {
		if got := fileIconType(tc.path, tc.directory); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.path, got, tc.want)
		}
	}
}

func TestEmbeddedSystemIcons(t *testing.T) {
	for _, name := range []string{"app", "folder", "applications", "file", "text", "pdf", "image", "audio", "video", "archive", "disk", "package", "font", "script", "source", "executable", "alias"} {
		data, err := fileIconFiles.ReadFile("assets/fileicons/" + name + ".png")
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
			t.Fatalf("%s: expected 256px icon", name)
		}
		visible := false
		for y := range 256 {
			for x := range 256 {
				_, _, _, a := img.At(x, y).RGBA()
				visible = visible || a != 0
			}
		}
		if !visible {
			t.Fatalf("%s: empty icon", name)
		}
	}
}
