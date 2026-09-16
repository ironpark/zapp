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
		g.s.Project.DMG.Contents[fmt.Sprintf("file-%02d", i)] = zapp.Content{X: &x, Y: &y}
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
	if len(g.inspector.Inputs) != 3 {
		t.Fatal("missing inspector fields")
	}
	index := g.inspectorStart + 1
	g.focus(index)
	g.input.SetText("222")
	if !g.commit() || *g.s.Project.DMG.Contents[g.selected].X != 222 {
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
