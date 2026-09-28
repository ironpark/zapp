package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ironpark/zapp"
)

// Artifact paths are appended as absolute key=value lines, so the file can be
// $GITHUB_OUTPUT itself, and a step that was not built is empty.
func TestWriteArtifacts(t *testing.T) {
	t.Chdir(t.TempDir())
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(cwd, "output")
	if err := os.WriteFile(file, []byte("earlier=kept\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeArtifacts(file, zapp.Artifacts{App: "MyApp.app", DMG: filepath.Join("dist", "MyApp.dmg")}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	want := "earlier=kept\n" +
		"app=" + filepath.Join(cwd, "MyApp.app") + "\n" +
		"dmg=" + filepath.Join(cwd, "dist", "MyApp.dmg") + "\n" +
		"pkg=\n"
	if string(data) != want {
		t.Fatalf("output =\n%s\nwant\n%s", data, want)
	}
}
