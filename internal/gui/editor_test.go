package gui

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func testEditor(t *testing.T) *editor {
	t.Helper()
	s := fixture(t, ".zapp.yaml", "version: 1\ndmg: {}\npkg: {}\n")
	g := &editor{s: s, w: 1200, h: 840, active: -1, assets: map[string]*ebiten.Image{}}
	g.rebuild()
	return g
}

func TestTabComponentRejectsNavigationWithInvalidDraft(t *testing.T) {
	g := testEditor(t)
	g.tab = 1
	g.rebuild()
	for i, f := range g.fields {
		if f.Label == "Icon size" {
			g.focus(i)
			break
		}
	}
	g.input.SetText("8")
	tabs := g.tabs()
	target := tabs.Buttons()[2].Bounds.Min.Add(image.Pt(2, 2))
	if !tabs.Click(target) {
		t.Fatal("tab click was not consumed")
	}
	if g.tab != 1 || g.active < 0 || g.input.Text() != "8" {
		t.Fatal("invalid draft lost during tab transition")
	}
	g.input.SetText("96")
	g.tabs().Click(target)
	if g.tab != 2 || g.s.Project.DMG.IconSize != 96 {
		t.Fatal("valid draft did not apply before navigation")
	}
}

func TestFieldCommitAndUndo(t *testing.T) {
	g := testEditor(t)
	g.focus(0)
	g.input.SetText("dist/MyApp.app")
	if !g.commit() || g.s.Project.App != "dist/MyApp.app" {
		t.Fatal("app field was not applied")
	}
	g.tab = 2
	g.rebuild()
	for i, f := range g.fields {
		if f.Label == "Identifier" {
			g.focus(i)
			g.input.SetText("dev.example.app")
			if !g.commit() {
				t.Fatal("PKG identifier was not applied")
			}
			break
		}
	}
	if g.s.Project.PKG.Identifier != "dev.example.app" || g.s.Project.App != "dist/MyApp.app" {
		t.Fatal("tab edit lost another setting")
	}
	g.s.Undo()
	if g.s.Project.PKG.Identifier != "" || g.s.Project.App != "dist/MyApp.app" {
		t.Fatal("undo did not isolate the last change")
	}
}

func TestInvalidFieldRetainsDraft(t *testing.T) {
	g := testEditor(t)
	g.tab = 1
	g.rebuild()
	for i, f := range g.fields {
		if f.Label != "Icon size" {
			continue
		}
		g.focus(i)
		g.input.SetText("8")
		g.input.SetCursor(1)
		if g.commit() {
			t.Fatal("accepted too-small icon")
		}
		if g.active < 0 || g.input.Text() != "8" || g.s.Project.DMG.IconSize != 0 {
			t.Fatal("invalid edit was lost or applied")
		}
		g.input.SetText("96")
		g.input.SetCursor(2)
		if !g.commit() || g.s.Project.DMG.IconSize != 96 {
			t.Fatal("could not correct invalid draft")
		}
		return
	}
	t.Fatal("missing icon size field")
}

func TestToggleRestoresDisabledSettings(t *testing.T) {
	g := testEditor(t)
	g.tab = 2
	g.s.Project.PKG.Identifier = "dev.example.app"
	g.toggle()
	if g.s.Project.PKG != nil {
		t.Fatal("PKG was not disabled")
	}
	g.toggle()
	if g.s.Project.PKG == nil || g.s.Project.PKG.Identifier != "dev.example.app" {
		t.Fatal("toggle lost fields")
	}
}

func TestDefaultIconInspectorDoesNotMutateOnSelection(t *testing.T) {
	g := testEditor(t)
	g.s.Project.App = "Demo.app"
	g.tab = 1
	g.selected = "Demo.app"
	g.rebuild()
	if g.s.Project.DMG.Contents != nil {
		t.Fatal("selecting an icon must not materialize contents")
	}
	for i, f := range g.fields {
		if f.Label == "X" {
			g.focus(i)
			g.input.SetText("210")
			if !g.commit() {
				t.Fatal("could not edit default icon")
			}
			if *g.s.Project.DMG.Contents["Demo.app"].X != 210 || !g.s.Project.DMG.Contents["/Applications"].Link {
				t.Fatal("inspector lost layout")
			}
			return
		}
	}
	t.Fatal("missing default icon inspector")
}

func TestAdvancedChoice(t *testing.T) {
	g := testEditor(t)
	g.tab = 1
	g.rebuild()
	for _, f := range g.fields {
		if f.Label == "Filesystem" {
			t.Fatal("advanced field visible by default")
		}
	}
	g.dmgAdvanced = true
	g.rebuild()
	for i, f := range g.fields {
		if f.Label == "Filesystem" {
			g.focus(i)
			g.openChoice()
			g.choose(1)
			break
		}
	}
	if g.s.Project.DMG.FS != "hfsplus" {
		t.Fatal("choice did not apply")
	}

}

func TestActualPreviewPanPreservesProjectAndCoordinates(t *testing.T) {
	g := testEditor(t)
	g.previewActual = true
	g.pan = image.Pt(10000, -10000)
	tr := g.transform()
	if tr.scale != 1 {
		t.Fatal("actual size must use unit scale")
	}
	x, y := tr.content(tr.x+120, tr.y+80)
	if x != 120 || y != 80 {
		t.Fatal("pan changed icon coordinate mapping")
	}
	if g.pan.X == 10000 || g.pan.Y == -10000 {
		t.Fatal("pan must be clamped to content")
	}
	if g.s.Dirty() || len(g.s.undo) != 0 {
		t.Fatal("view changes modified project")
	}
	g.previewActual = false
	fit := g.transform()
	if !fit.bounds.In(g.previewArea()) {
		t.Fatal("fit mode must contain the canvas")
	}
}
