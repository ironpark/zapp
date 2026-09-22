package gui

import (
	"image"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/ironpark/zapp/internal/gui/comp"
)

// tick is the per-frame input snapshot. The pointer is sampled once: pos is the
// live position that drag and pan follow, while mouse is where a click is
// routed — the press position, which can differ from pos on a fast click.
type tick struct {
	pos, mouse image.Point
	click      bool
}

// history keeps toolbar and keyboard feedback consistent after restoring state.
func (g *editor) history(redo bool) {
	if g.active >= 0 {
		g.rebuild()
		g.report(nil, "Edit cancelled.")
		return
	}
	message := "Nothing to undo."
	if redo {
		message = "Nothing to redo."
		if g.s.CanRedo() {
			g.s.Redo()
			message = "Redo applied."
		}
	} else if g.s.CanUndo() {
		g.s.Undo()
		message = "Undo applied."
	}
	g.issue = nil
	g.rebuild()
	g.report(nil, message)
}

// selectPreviewItem picks the topmost icon under the pointer and begins a drag.
func (g *editor) selectPreviewItem(in tick) {
	t := g.transform()
	if !in.mouse.In(g.previewArea()) || !in.mouse.In(t.bounds) || (g.previewActual && ebiten.IsKeyPressed(ebiten.KeySpace)) {
		return
	}
	l := g.s.layout()
	x, y := t.content(float64(in.mouse.X), float64(in.mouse.Y))
	reach := float64(l.IconSize) / 2
	g.selected = ""
	for _, item := range slices.Backward(l.Items) {
		if math.Abs(x-float64(item.X)) <= reach && math.Abs(y-float64(item.Y)) <= reach {
			g.selected, g.drag = item.Path, item.Path
			g.dragX, g.dragY = x-float64(item.X), y-float64(item.Y)
			g.dragMoved = false
			break
		}
	}
	g.rebuild()
	g.revealItem()
}

// startPan begins a space-drag pan, which is only offered at actual size.
func (g *editor) startPan(in tick) {
	if !g.previewActual || !in.mouse.In(g.previewArea()) || g.drag != "" {
		return
	}
	g.panning = true
	g.panStart, g.panOrigin = in.mouse, g.pan
}

func (g *editor) nudgeSelected() {
	if g.tab != tabDMG || g.s.Project.DMG == nil || g.selected == "" {
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
	if item, ok := g.s.layout().find(g.selected); ok {
		g.s.checkpoint()
		g.s.move(item.Path, item.X+dx, item.Y+dy)
		g.rebuild()
	}
}
