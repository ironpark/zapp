package gui

import (
	"image"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/ironpark/zapp/internal/gui/comp"
)

const choiceRowHeight = 32

func (g *editor) openChoice() {
	if g.active < 0 || len(g.input.Spec.Choices) == 0 {
		return
	}
	if g.input.Spec.Boolean {
		value := "true"
		if g.input.Text() == "true" {
			value = "false"
		}
		g.input.SetText(value)
		g.commit()
		return
	}
	g.choiceOpen = true
	g.choiceIndex = max(0, slices.Index(g.input.Spec.Choices, g.input.Text()))
	g.form.SetBounds(g.formArea())
	g.revealField(g.active)
}
func (g *editor) closeChoice() {
	g.choiceOpen = false
	g.form.SetBounds(g.formArea())
}
func (g *editor) choiceBounds() image.Rectangle {
	form, index := g.fieldForm(g.active)
	field := form.FieldBounds(index)
	return comp.Box(field.Min.X, field.Max.Y+4, field.Dx(), len(g.input.Spec.Choices)*choiceRowHeight+4)
}
func (g *editor) choose(index int) {
	if index < 0 || index >= len(g.input.Spec.Choices) {
		return
	}
	g.input.SetText(g.input.Spec.Choices[index])
	g.closeChoice()
	g.commit()
}
func (g *editor) handleChoice(in tick, k comp.Keyboard) {
	if k.JustPressed(ebiten.KeyEscape) {
		g.closeChoice()
		return
	}
	if k.JustPressed(ebiten.KeyTab) {
		g.closeChoice()
		delta := 1
		if k.Shift {
			delta = -1
		}
		g.focus((g.active + delta + len(g.fields)) % len(g.fields))
		return
	}
	if k.Repeats(ebiten.KeyArrowDown) {
		g.choiceIndex = (g.choiceIndex + 1) % len(g.input.Spec.Choices)
	}
	if k.Repeats(ebiten.KeyArrowUp) {
		g.choiceIndex = (g.choiceIndex + len(g.input.Spec.Choices) - 1) % len(g.input.Spec.Choices)
	}
	if k.JustPressed(ebiten.KeyEnter) || k.JustPressed(ebiten.KeySpace) {
		g.choose(g.choiceIndex)
		return
	}
	if in.click {
		bounds := g.choiceBounds().Inset(2)
		if in.mouse.In(bounds) {
			g.choose((in.mouse.Y - bounds.Min.Y) / choiceRowHeight)
		} else {
			g.closeChoice()
		}
	}
}
func (g *editor) drawChoice(dst *ebiten.Image, pointer image.Point) {
	bounds := g.choiceBounds()
	p, t := g.ui, g.ui.Theme
	comp.Surface(dst, bounds, comp.Radius, t.Panel, t.Border)
	for i, value := range g.input.Spec.Choices {
		row := comp.Box(bounds.Min.X+2, bounds.Min.Y+2+i*choiceRowHeight, bounds.Dx()-4, choiceRowHeight)
		if i == g.choiceIndex || pointer.In(row) {
			comp.RoundedRect(dst, row, 4, t.Hover)
		}
		label := value
		if label == "" {
			label = "Default"
		}
		p.Text(dst, p.Fit(label, row.Dx()-40, 14), row.Min.X+10, row.Min.Y+6, 14, t.Text)
		if value == g.input.Text() {
			p.Text(dst, "✓", row.Max.X-24, row.Min.Y+6, 14, t.Accent)
		}
	}
}
