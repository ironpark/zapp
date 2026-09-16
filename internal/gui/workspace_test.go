package gui

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/ironpark/zapp"
)

func TestPickerResultRelativePathUndoAndCancel(t *testing.T) {
	g := testEditor(t)
	results := make(chan pickResult, 1)
	results <- pickResult{index: 0, path: filepath.Join(filepath.Dir(g.s.Path), "Demo.app")}
	g.picking = results
	g.pollPicker()
	if g.s.Project.App != "Demo.app" || g.picking != nil {
		t.Fatal("picker result did not commit a relative path")
	}
	g.s.Undo()
	g.rebuild()
	if g.s.Project.App != "" {
		t.Fatal("picked path could not undo")
	}
	g.focus(0)
	g.input.SetText("unfinished")
	results <- pickResult{index: 0}
	g.picking = results
	g.pollPicker()
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
	if g.commit() || g.input.Spec.Error == "" || g.fields[g.active].Error == "" {
		t.Fatal("invalid draft did not get inline error")
	}
	g.input.SetText("96")
	if !g.commit() || g.s.Project.DMG.IconSize != 96 {
		t.Fatal("corrected value not applied")
	}
	g.s.Project.App = "missing.app"
	g.rebuild()
	g.validate()
	if g.tab != tabProject || g.active < 0 || g.fields[g.active].Label != "App bundle" || g.input.Spec.Error == "" {
		t.Fatal("validation did not navigate to invalid app")
	}
}
func TestItemListInspectorStableAndScrollable(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.w = 1080
	g.h = 720
	g.s.Project.DMG.Contents = map[string]zapp.Content{}
	for i := 0; i < 12; i++ {
		x, y := 100+i, 120
		g.s.Project.DMG.Contents[fmt.Sprintf("file-%02d", i)] = zapp.Content{Pos: &zapp.Position{x, y}}
	}
	g.rebuild()
	g.form.ScrollTo(80)
	offset := g.form.Offset()
	title := g.form.FieldBounds(0)
	g.itemScroll = g.itemListLimit()
	if !g.selectListItem(g.itemsPanel().Content().Max.Sub(image.Pt(10, 10))) || g.selected == "" {
		t.Fatal("scrolled list selection failed")
	}
	if g.form.Offset() != offset || g.form.FieldBounds(0) != title {
		t.Fatal("selection shifted layout settings")
	}
	if len(g.inspector.Inputs) != 4 {
		t.Fatal("missing inspector fields")
	}
	index := g.inspectorStart + 1
	g.focus(index)
	g.input.SetText("222")
	if !g.commit() || g.s.Project.DMG.Contents[g.selected].Pos[0] != 222 {
		t.Fatal("inspector edit not applied")
	}
	if g.previewArea().Overlaps(g.itemsPanel().Bounds) || g.previewPanel().Bounds.Overlaps(g.inspectorPanel().Bounds) {
		t.Fatal("preview overlaps inspector")
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

func TestWorkspaceSizesKeepControlsAndInspectorUsable(t *testing.T) {
	for _, size := range []image.Point{{1080, 720}, {1200, 840}, {1600, 1000}} {
		g := testEditor(t)
		g.tab = tabDMG
		g.s.Project.App = "Example.app"
		g.w, g.h = size.X, size.Y
		g.selected = "Example.app"
		g.rebuild()
		if g.inspector.Limit() != 0 {
			t.Fatalf("%v: ordinary item fields require scrolling", size)
		}
		if !g.previewArea().In(g.previewPanel().Bounds) || g.previewArea().Dx() < 350 {
			t.Fatalf("%v: unusable preview bounds", size)
		}
		buttons := g.controls()
		for i, button := range buttons {
			if !button.Bounds.In(image.Rect(0, 0, g.w, g.h)) {
				t.Fatalf("%v: offscreen %s", size, button.Label)
			}
			for _, other := range buttons[i+1:] {
				if button.Bounds.Overlaps(other.Bounds) {
					t.Fatalf("%v: %s overlaps %s", size, button.Label, other.Label)
				}
			}
		}
		for tab := range sections {
			g.switchTab(tab)
			tabs := g.tabs().Buttons()
			if len(tabs) != len(sections) {
				t.Fatalf("%v: tabs were clipped", size)
			}
			for _, tabButton := range tabs {
				if g.section().Optional() && tabButton.Bounds.Overlaps(g.stepToggle().Bounds) {
					t.Fatalf("%v: tab overlaps enable toggle", size)
				}
				for _, action := range g.controls() {
					if tabButton.Bounds.Overlaps(action.Bounds) {
						t.Fatalf("%v: tab overlaps %s", size, action.Label)
					}
				}
			}
			if g.section().Optional() {
				for _, action := range g.controls() {
					if action.Bounds.Overlaps(g.stepToggle().Bounds) {
						t.Fatalf("%v: %s overlaps enable toggle", size, action.Label)
					}
				}
			}
		}
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
