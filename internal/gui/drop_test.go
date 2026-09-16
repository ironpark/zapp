package gui

import (
	"image"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// Only actual dropped entries expose AbsPath on their opened file. Grouping
// directories must be traversed without accidentally adding their whole tree.
type dropTestFS struct {
	fs.FS
	paths map[string]string
}
type dropTestFile struct {
	fs.File
	absolute string
}

func (f dropTestFile) AbsPath() string { return f.absolute }
func (f dropTestFS) Open(name string) (fs.File, error) {
	file, err := f.FS.Open(name)
	if err != nil {
		return nil, err
	}
	if absolute, ok := f.paths[name]; ok {
		return dropTestFile{file, absolute}, nil
	}
	return file, nil
}
func TestDroppedPathsPreservesDirectoryAndTraversesGroups(t *testing.T) {
	files := dropTestFS{fstest.MapFS{
		"group/a.txt":           &fstest.MapFile{Data: []byte("a")},
		"App.app/Contents/info": &fstest.MapFile{Data: []byte("bundle")},
	}, map[string]string{"group/a.txt": "/tmp/group/a.txt", "App.app": "/tmp/App.app"}}
	paths, err := droppedPaths(files)
	if err != nil || len(paths) != 2 || paths[0] != "/tmp/App.app" || paths[1] != "/tmp/group/a.txt" {
		t.Fatalf("paths=%v err=%v", paths, err)
	}
}
func TestDropBatchCoordinatesDuplicatesAndUndo(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.rebuild()
	dir := filepath.Dir(g.s.Path)
	file := filepath.Join(dir, "Readme.txt")
	folder := filepath.Join(dir, "Extras")
	if err := os.WriteFile(file, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	transform := g.transform()
	point := image.Pt(int(transform.x+100*transform.scale), int(transform.y+100*transform.scale))
	before := len(layout(g.s.Project.DMG, g.s.Project.App).Items)
	n, err := g.addDroppedPaths([]string{file, folder, file}, point)
	if err != nil || n != 2 {
		t.Fatalf("count=%d err=%v", n, err)
	}
	if len(layout(g.s.Project.DMG, g.s.Project.App).Items) != before+2 {
		t.Fatal("drop lost existing items")
	}
	content := g.s.Project.DMG.Contents["Readme.txt"]
	if content.X == nil || *content.X < 98 || *content.X > 101 {
		t.Fatalf("drop was not positioned in preview coordinates: %+v", content)
	}
	if n, err := g.addDroppedPaths([]string{file}, point); n != 0 || err != nil {
		t.Fatal("duplicate drop changed layout")
	}
	if !g.s.CanUndo() {
		t.Fatal("batch could not undo")
	}
	g.s.Undo()
	if g.s.Project.DMG.Contents != nil {
		t.Fatal("one undo did not restore automatic layout")
	}
	if n, err := g.addDroppedPaths([]string{file}, image.Pt(0, 0)); n != 0 || err != nil {
		t.Fatal("drop outside preview accepted")
	}
}
func TestDropRejectsInvalidBatchWithoutPartialChanges(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.rebuild()
	dir := t.TempDir()
	first := filepath.Join(dir, "a", "same.txt")
	second := filepath.Join(dir, "b", "same.txt")
	for _, p := range []string{first, second} {
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	_, err := g.addDroppedPaths([]string{first, second}, g.previewArea().Min.Add(image.Pt(100, 100)))
	if err == nil || g.s.Project.DMG.Contents != nil || g.s.CanUndo() {
		t.Fatal("invalid batch was partially applied")
	}
}
