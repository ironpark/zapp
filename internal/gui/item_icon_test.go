package gui

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/ironpark/zapp"
)

func TestItemIconLivePreviewAndReset(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.s.Project.DMG.Contents = map[string]zapp.Content{"notes.txt": {Pos: &zapp.Position{180, 200}}}
	g.selected = "notes.txt"
	g.rebuild()
	original := g.derivedSignature()
	if g.fields[g.inspectorStart+3].picker != pickItemIcon || !slices.Contains(pickerExtensions(pickItemIcon), "jpg") || !slices.Contains(pickerExtensions(pickItemIcon), "jpeg") {
		t.Fatal("missing JPEG picker filter")
	}
	g.focus(g.inspectorStart + 3)
	icon, err := filepath.Abs("assets/fileicons/folder.png")
	if err != nil {
		t.Fatal(err)
	}
	g.input.SetText(icon)
	g.previewInput()
	if g.assets["item:notes.txt"] == nil || g.derivedSignature() == original {
		t.Fatal("custom icon not previewed")
	}
	if !g.commit() {
		t.Fatal("icon did not commit")
	}
	if g.s.Project.DMG.Contents[g.selected].Icon != icon {
		t.Fatal("icon path not stored")
	}
	g.toggleItemLink()
	if g.s.Project.DMG.Contents[g.selected].Link {
		t.Fatal("custom icon allowed on link")
	}
	found := false
	for _, button := range g.controls() {
		if button.Label == "Reset item icon" {
			found = true
			button.OnClick()
			break
		}
	}
	if !found || g.s.Project.DMG.Contents[g.selected].Icon != "" {
		t.Fatal("reset failed")
	}
	g.history(false)
	if g.s.Project.DMG.Contents[g.selected].Icon != icon {
		t.Fatal("reset did not undo")
	}
	g.history(false)
	if g.s.Project.DMG.Contents[g.selected].Icon != "" {
		t.Fatal("icon change did not undo")
	}
	g.toggleItemLink()
	if len(g.inspector.Inputs) != 3 {
		t.Fatal("link shows unsupported icon picker")
	}
}
