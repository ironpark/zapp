package gui

import (
	"image"
	"math"
	"slices"
)

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
func (g *editor) selectPreviewItem(point image.Point) {
	t := g.transform()
	if !point.In(g.previewArea()) || !point.In(t.bounds) {
		return
	}
	l := g.s.layout()
	x, y := t.content(float64(point.X), float64(point.Y))
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
}

// startPan begins a space-drag pan, which is only offered at actual size.
func (g *editor) startPan(point image.Point) {
	if !g.previewActual || !point.In(g.previewArea()) || g.drag != "" {
		return
	}
	g.panning = true
	g.panStart, g.panOrigin = point, g.pan
}
