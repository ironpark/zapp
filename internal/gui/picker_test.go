package gui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPickerDirectory(t *testing.T) {
	base := t.TempDir()
	app := filepath.Join(base, "Demo.app")
	if err := os.Mkdir(app, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(base, "image.png")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, value string
		mode        pickMode
	}{
		{"empty app", "", pickApp},
		{"empty file", "", pickFile},
		{"relative file", "image.png", pickFile},
		{"absolute file", file, pickFile},
		{"missing nested output", "not-created/nested/output.dmg", pickSave},
		{"expression", "${env:BACKGROUND}", pickFile},
		{"app bundle", app, pickApp},
		{"existing folder", base, pickFolder},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := pickerDirectory(filepath.Join(base, ".zapp.yaml"), tt.value, tt.mode)
			if got != base || !filepath.IsAbs(got) {
				t.Fatalf("got %q, want absolute %q", got, base)
			}
		})
	}
}

func TestPickedPathThroughSymlinkDirectory(t *testing.T) {
	root := t.TempDir()
	actual := filepath.Join(root, "actual")
	if err := os.Mkdir(actual, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(actual, alias); err != nil {
		t.Skip(err)
	}
	config := filepath.Join(alias, ".zapp.yaml")
	real, err := filepath.EvalSymlinks(actual)
	if err != nil {
		t.Fatal(err)
	}
	if got := pickedPath(config, filepath.Join(real, "new-output.dmg")); got != "new-output.dmg" {
		t.Fatalf("unexpected canonicalized path %q", got)
	}
}

func TestAddFilePickerPreservesSettingsAndSupportsUndo(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.rebuild()
	g.form.ScrollTo(80)
	beforeOffset := g.form.Offset()
	beforeTitle := g.form.FieldBounds(0)
	file := filepath.Join(filepath.Dir(g.s.Path), "Readme.txt")
	if err := os.WriteFile(file, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	results := make(chan pickResult, 1)
	results <- pickResult{index: addItemPicker}
	g.picking = results
	g.pollPicker()
	if g.s.Project.DMG.Contents != nil || g.form.Offset() != beforeOffset {
		t.Fatal("cancel changed layout")
	}
	results <- pickResult{index: addItemPicker, path: file}
	g.picking = results
	g.pollPicker()
	if _, ok := g.s.Project.DMG.Contents["Readme.txt"]; !ok {
		t.Fatal("picked file not added")
	}
	if g.form.Offset() != beforeOffset || g.form.FieldBounds(0) != beforeTitle {
		t.Fatal("add shifted layout settings")
	}
	if g.selected != "Readme.txt" {
		t.Fatal("new file not selected")
	}
	results <- pickResult{index: addItemPicker, path: file}
	g.picking = results
	g.pollPicker()
	if g.failed {
		t.Fatal("duplicate should be skipped")
	}
	g.s.Undo()
	if g.s.Project.DMG.Contents != nil {
		t.Fatal("addition did not undo as one change")
	}
}

func TestImagePickerFilters(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.dmgAdvanced = true
	g.rebuild()
	for _, f := range g.fields {
		switch f.Label {
		case "Background image":
			if f.picker != pickImage || len(pickerExtensions(f.picker)) != 3 {
				t.Fatal("missing PNG/JPEG filter")
			}
		case "Disk icon":
			if f.picker != pickIcon || len(pickerExtensions(f.picker)) != 2 {
				t.Fatal("missing ICNS/PNG filter")
			}
		}
	}
	if len(pickerExtensions(pickFile)) != 0 {
		t.Fatal("generic file picker must stay unrestricted")
	}
}
