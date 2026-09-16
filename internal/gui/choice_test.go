package gui

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/ironpark/zapp/internal/gui/comp"
)

func TestChoiceDropdownSelectCancelAndKeyboard(t *testing.T) {
	g := testEditor(t)
	g.w = 1080
	g.h = 720
	g.tab = tabDMG
	g.dmgAdvanced = true
	g.rebuild()
	index := -1
	for i, f := range g.fields {
		if f.Label == "Filesystem" {
			index = i
			break
		}
	}
	if index < 0 {
		t.Fatal("missing filesystem")
	}
	g.focus(index)
	g.openChoice()
	bounds := g.choiceBounds()
	if bounds.Min.Y <= g.form.FieldBounds(index).Max.Y || bounds.Max.Y > g.settingsPanel().Bounds.Max.Y {
		t.Fatal("dropdown not below field within panel")
	}
	g.handleChoice(tick{}, comp.Keyboard{Pressed: []ebiten.Key{ebiten.KeyArrowDown}})
	if g.s.Project.DMG.FS != "" {
		t.Fatal("navigation committed value")
	}
	g.handleChoice(tick{}, comp.Keyboard{Pressed: []ebiten.Key{ebiten.KeyEscape}})
	if g.choiceOpen || g.s.Project.DMG.FS != "" {
		t.Fatal("escape did not cancel")
	}
	g.openChoice()
	bounds = g.choiceBounds()
	g.handleChoice(tick{click: true, mouse: bounds.Min.Add(image.Pt(10, 2+2*choiceRowHeight+10))}, comp.Keyboard{})
	if g.choiceOpen || g.s.Project.DMG.FS != "apfs" {
		t.Fatal("click did not select apfs")
	}
	g.focus(index)
	g.openChoice()
	g.handleChoice(tick{}, comp.Keyboard{Pressed: []ebiten.Key{ebiten.KeyArrowUp}})
	g.handleChoice(tick{}, comp.Keyboard{Pressed: []ebiten.Key{ebiten.KeyEnter}})
	if g.s.Project.DMG.FS != "hfsplus" {
		t.Fatal("keyboard selection failed")
	}
	g.focus(index)
	g.openChoice()
	g.handleChoice(tick{click: true, mouse: image.Pt(0, 0)}, comp.Keyboard{})
	if g.choiceOpen || g.s.Project.DMG.FS != "hfsplus" {
		t.Fatal("outside click changed selection")
	}
}
