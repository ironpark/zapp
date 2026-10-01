package gui

import (
	"image"
	"os"
	"path/filepath"
	"testing"
)

func TestDropBatchCoordinatesDuplicatesAndUndo(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.previewBounds = image.Rect(0, 0, 800, 600)
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
	before := len(g.s.layout().Items)
	n, err := g.addDroppedPaths([]string{file, folder, file}, point)
	if err != nil || n != 2 {
		t.Fatalf("count=%d err=%v", n, err)
	}
	if len(g.s.layout().Items) != before+2 {
		t.Fatal("drop lost existing items")
	}
	content := g.s.Project.DMG.Contents["Readme.txt"]
	if content.Pos == nil || content.Pos[0] < 98 || content.Pos[0] > 101 {
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
	if n, err := g.addDroppedPaths([]string{file}, image.Pt(900, 700)); n != 0 || err != nil {
		t.Fatal("drop outside preview accepted")
	}
}
func TestDropRejectsInvalidBatchWithoutPartialChanges(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.previewBounds = image.Rect(0, 0, 800, 600)
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
