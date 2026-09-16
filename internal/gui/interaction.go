package gui

import (
	"image"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/ironpark/zapp/internal/gui/comp"
)

// tick is the per-frame input snapshot. The pointer is sampled once: pos is the
// live position that drag and pan follow, while mouse is where a click is
// routed — the press position, which can differ from pos on a fast click.
type tick struct {
	pos, mouse image.Point
	click      bool
}

func captureTick(w, h int) tick {
	px, py := comp.PointerPosition(w, h)
	in := tick{pos: image.Pt(px, py), click: inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)}
	in.mouse = in.pos
	if in.click {
		mx, my := comp.PointerPressPosition(w, h)
		in.mouse = image.Pt(mx, my)
	}
	return in
}

// tabKeys select a tab by position with the Command/Ctrl modifier held.
var tabKeys = []ebiten.Key{ebiten.KeyDigit1, ebiten.KeyDigit2, ebiten.KeyDigit3, ebiten.KeyDigit4, ebiten.KeyDigit5, ebiten.KeyDigit6}

// handleShortcuts runs the Command/Ctrl chords, reporting whether one fired and
// consumed the tick.
func (g *editor) handleShortcuts() bool {
	if !comp.CommandKey() {
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyS) {
		g.save()
		return true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) {
		switch {
		case g.active >= 0:
			// An edit is in progress: discard the draft, not the last change.
		case ebiten.IsKeyPressed(ebiten.KeyShift):
			g.s.Redo()
		default:
			g.s.Undo()
		}
		g.rebuild()
		return true
	}
	for i, key := range tabKeys {
		if inpututil.IsKeyJustPressed(key) {
			g.switchTab(i)
			return true
		}
	}
	return false
}

// handleTyping routes the keyboard to the focused field, to focus movement, or
// to the selected preview icon, depending on what currently holds focus.
func (g *editor) handleTyping() {
	switch {
	case g.active >= 0:
		g.editInput()
	case inpututil.IsKeyJustPressed(ebiten.KeyTab) && len(g.fields) > 0:
		g.focus(0)
	default:
		g.nudgeSelected()
	}
}

// handleClick routes a press to the chrome, the form or the preview. It reports
// whether the click was consumed and the rest of the tick should stop; starting
// a drag or a pan does not consume it, so both continue in the same tick.
func (g *editor) handleClick(in tick) bool {
	if g.tabs().Click(in.mouse) || (g.section().Optional() && g.stepToggle().Click(in.mouse)) || comp.ClickButtons(g.controls(), in.mouse) {
		return true
	}
	if i, ok := g.form.Hit(in.mouse); ok {
		if g.active != i {
			if !g.commit() {
				return true
			}
			g.focus(i)
		}
		if g.active >= 0 && len(g.input.Spec.Choices) > 0 {
			g.cycleChoice(g.active)
		}
		return true
	}
	if !g.commit() {
		return true
	}
	if g.tab == tabDMG && g.enabled() {
		g.selectPreviewItem(in)
		g.startPan(in)
	}
	return false
}

// selectPreviewItem picks the topmost icon under the pointer and begins a drag.
func (g *editor) selectPreviewItem(in tick) {
	t := g.transform()
	if !in.mouse.In(g.previewArea()) || !in.mouse.In(t.bounds) || (g.previewActual && ebiten.IsKeyPressed(ebiten.KeySpace)) {
		return
	}
	l := layout(g.s.Project.DMG, g.s.Project.App)
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
	g.form.ScrollTo(0)
}

// startPan begins a space-drag pan, which is only offered at actual size.
func (g *editor) startPan(in tick) {
	if !g.previewActual || !in.mouse.In(g.previewArea()) || g.drag != "" {
		return
	}
	g.panning = true
	g.panStartX, g.panStartY = in.mouse.X, in.mouse.Y
	g.panOriginX, g.panOriginY = g.panX, g.panY
}

func (g *editor) updatePan(in tick) {
	if !g.panning {
		return
	}
	g.panX = g.panOriginX + in.pos.X - g.panStartX
	g.panY = g.panOriginY + in.pos.Y - g.panStartY
	g.clampPan()
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		g.panning = false
	}
}

func (g *editor) updateDrag(in tick) {
	if g.drag == "" {
		return
	}
	// Apply the release position too: a short drag can finish between ticks.
	x, y := g.transform().content(float64(in.pos.X), float64(in.pos.Y))
	x, y = x-g.dragX, y-g.dragY
	nx, ny := int(math.Round(x)), int(math.Round(y))
	if i, ok := layout(g.s.Project.DMG, g.s.Project.App).find(g.drag); ok && (nx != i.X || ny != i.Y) {
		if !g.dragMoved {
			g.s.checkpoint()
			g.dragMoved = true
		}
		g.s.move(g.drag, nx, ny)
	}
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		g.drag = ""
		if g.dragMoved {
			g.rebuild()
			g.report(nil, "Position updated. Arrow keys nudge; Shift moves 10 pixels.")
		}
	}
}

func (g *editor) scrollForm(in tick) {
	if in.mouse.In(g.form.Bounds) {
		_, wheel := ebiten.Wheel()
		g.form.ScrollBy(-int(wheel * 38))
	}
}

func (g *editor) cycleChoice(i int) { g.focus(i); g.input.NextChoice(); g.commit() }
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
	if item, ok := layout(g.s.Project.DMG, g.s.Project.App).find(g.selected); ok {
		g.s.checkpoint()
		g.s.move(item.Path, item.X+dx, item.Y+dy)
		g.rebuild()
	}
}
