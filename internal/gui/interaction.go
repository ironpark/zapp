package gui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/ironpark/zapp/internal/gui/comp"
)

func (g *editor) cycleChoice(i int) { g.focus(i); g.input.NextChoice(); g.commit() }
func (g *editor) nudgeSelected() {
	if g.tab != 1 || g.s.Project.DMG == nil || g.selected == "" {
		return
	}
	dx, dy := 0, 0
	if comp.KeyRepeated(ebiten.KeyArrowLeft) {
		dx--
	}
	if comp.KeyRepeated(ebiten.KeyArrowRight) {
		dx++
	}
	if comp.KeyRepeated(ebiten.KeyArrowUp) {
		dy--
	}
	if comp.KeyRepeated(ebiten.KeyArrowDown) {
		dy++
	}
	if dx == 0 && dy == 0 {
		return
	}
	if ebiten.IsKeyPressed(ebiten.KeyShift) {
		dx *= 10
		dy *= 10
	}
	if item, ok := layout(g.s.Project.DMG, g.s.Project.App).find(g.selected); ok {
		g.s.checkpoint()
		g.s.move(item.Path, item.X+dx, item.Y+dy)
		g.rebuild()
	}
}
