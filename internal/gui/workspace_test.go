package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ironpark/zapp"
)

func TestPickerResultRelativePathUndoAndCancel(t *testing.T) {
	g := testEditor(t)
	g.picked(0, filepath.Join(filepath.Dir(g.s.Path), "Demo.app"), nil)
	if g.s.Project.App != "Demo.app" {
		t.Fatal("picker result did not commit a relative path")
	}
	g.s.Undo()
	g.rebuild()
	if g.s.Project.App != "" {
		t.Fatal("picked path could not undo")
	}
	g.focus(0)
	g.input.SetText("unfinished")
	g.picked(0, "", nil)
	if g.input.Text() != "unfinished" || g.s.Project.App != "" {
		t.Fatal("cancel lost draft or changed project")
	}
}
func TestInvalidFieldErrorAndValidationNavigation(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.rebuild()
	for i, f := range g.fields {
		if f.Label == "Icon size" {
			g.focus(i)
			break
		}
	}
	g.input.SetText("bad")
	if g.commit() || g.fields[g.active].shownError() == "" {
		t.Fatal("invalid draft did not get inline error")
	}
	g.input.SetText("96")
	if !g.commit() || g.s.Project.DMG.IconSize != 96 {
		t.Fatal("corrected value not applied")
	}
	g.s.Project.App = "missing.app"
	g.rebuild()
	g.validate()
	if g.tab != tabProject || g.active < 0 || g.fields[g.active].Label != "App bundle" || g.fields[g.active].shownError() == "" {
		t.Fatal("validation did not navigate to invalid app")
	}
}
func TestItemSelectionKeepsSettingsAndEditsInspector(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.s.Project.DMG.Contents = map[string]zapp.Content{}
	for i := 0; i < 12; i++ {
		x, y := 100+i, 120
		g.s.Project.DMG.Contents[fmt.Sprintf("file-%02d", i)] = zapp.Content{Pos: &zapp.Position{x, y}}
	}
	g.rebuild()
	before := settingsLabels(g)
	g.selected = "file-11"
	g.rebuild()
	if !slices.Equal(settingsLabels(g), before) {
		t.Fatal("selection changed layout settings")
	}
	if len(g.fields)-g.inspectorStart != 4 {
		t.Fatal("missing inspector fields")
	}
	index := g.inspectorStart + 1
	g.focus(index)
	g.input.SetText("222")
	if !g.commit() || g.s.Project.DMG.Contents[g.selected].Pos[0] != 222 {
		t.Fatal("inspector edit not applied")
	}
}
func TestPickerPathValidationDoesNotBlockSavingDraft(t *testing.T) {
	g := testEditor(t)
	g.s.Project.App = "not-created.app"
	g.rebuild()
	if !g.save() {
		t.Fatal("missing input should still permit draft save")
	}
	file := filepath.Join(filepath.Dir(g.s.Path), "background.png")
	if err := os.Mkdir(file, 0700); err != nil {
		t.Fatal(err)
	}
	g.s.Project.App = ""
	g.s.Project.DMG.Background = "background.png"
	g.rebuild()
	if g.validatePaths() || g.tab != tabDMG || g.fields[g.active].Label != "Background image" {
		t.Fatal("directory accepted as image")
	}
}

func TestRemoveItemPreservesSourceAndUndo(t *testing.T) {
	g := testEditor(t)
	path := filepath.Join(filepath.Dir(g.s.Path), "source.txt")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	g.tab = tabDMG
	g.s.Project.DMG.Contents = map[string]zapp.Content{path: {}}
	g.selected = path
	g.rebuild()
	g.removeSelected()
	if len(g.s.layout().Items) != 0 || g.selected != "" {
		t.Fatal("item was not removed")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
		t.Fatal("source file changed")
	}
	g.history(false)
	if _, ok := g.s.Project.DMG.Contents[path]; !ok || g.status != "Undo applied." {
		t.Fatal("undo failed to restore item and feedback")
	}
}

func TestItemLinkTogglePreservesDetailsAndUndo(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	x, y := 120, 180
	g.s.Project.DMG.Contents = map[string]zapp.Content{"source": {Name: "Shortcut", Pos: &zapp.Position{x, y}}}
	g.selected = "source"
	g.rebuild()
	g.toggleItemLink()
	item := g.s.Project.DMG.Contents["source"]
	if !item.Link || item.Name != "Shortcut" || item.Pos[0] != x || item.Pos[1] != y || g.itemKinds["source"] != "Link" {
		t.Fatal("link conversion lost details or did not refresh the list")
	}
	g.history(false)
	if g.s.Project.DMG.Contents["source"].Link {
		t.Fatal("undo did not restore copy mode")
	}
	g.history(true)
	if !g.s.Project.DMG.Contents["source"].Link {
		t.Fatal("redo did not restore link mode")
	}
	g.toggleItemLink()
	if g.s.Project.DMG.Contents["source"].Link {
		t.Fatal("toggle did not return to copy mode")
	}
}

func TestItemLinkToggleMaterializesDefaultLayout(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.s.Project.App = "Example.app"
	g.selected = "/Applications"
	g.rebuild()
	g.toggleItemLink()
	if len(g.s.Project.DMG.Contents) != 2 || g.s.Project.DMG.Contents["/Applications"].Link {
		t.Fatal("default layout was not preserved when changing link mode")
	}
	g.history(false)
	if g.s.Project.DMG.Contents != nil {
		t.Fatal("undo did not restore automatic layout")
	}
}
