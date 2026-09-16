package gui

import (
	"context"
	"image"
	"math"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/ironpark/zapp"
	"github.com/ironpark/zapp/internal/gui/comp"
)

type editor struct {
	ctx                                      context.Context
	s                                        *Session
	disabled                                 zapp.Project
	w, h, tab                                int
	fields                                   []field
	active                                   int
	input                                    comp.Input
	form                                     comp.Form
	ui                                       *comp.Painter
	status                                   string
	failed, confirmClose, quit, projectDirty bool
	selected, newPath                        string
	drag                                     string
	dragX, dragY                             float64
	dragMoved                                bool
	assets                                   map[string]*ebiten.Image
	previewError                             string
	dmgAdvanced, adding                      bool
	previewActual, panning                   bool
	panX, panY, panStartX, panStartY         int
	panOriginX, panOriginY                   int
}

func Run(ctx context.Context, s *Session) error {
	painter, err := comp.NewPainter(nil, comp.DarkTheme())
	if err != nil {
		return err
	}
	// Prefer a local Unicode font when available, with the bundled font as fallback.
	for _, name := range []string{"/System/Library/Fonts/Supplemental/Arial Unicode.ttf", "C:/Windows/Fonts/malgun.ttf", "/usr/share/fonts/truetype/nanum/NanumGothic.ttf"} {
		if data, e := os.ReadFile(name); e == nil {
			if next, e := comp.NewPainter(data, comp.DarkTheme()); e == nil {
				painter.Close()
				painter = next
				break
			}
		}
	}
	defer painter.Close()
	g := &editor{ctx: ctx, s: s, w: 1200, h: 840, active: -1, ui: painter, assets: map[string]*ebiten.Image{}, status: "Edit settings, then Save. Validation checks build inputs without building."}
	g.rebuild()
	ebiten.SetWindowTitle("Zapp — Project settings")
	ebiten.SetWindowSize(g.w, g.h)
	ebiten.SetWindowSizeLimits(1080, 720, -1, -1)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowClosingHandled(true)
	stopPointer := comp.ObservePointer()
	defer stopPointer()
	return ebiten.RunGame(g)
}
func (g *editor) Layout(w, h int) (int, int) {
	g.w = w
	g.h = h
	g.form.SetBounds(g.formArea())
	return w, h
}
func (g *editor) report(err error, success string) {
	g.failed = err != nil
	if err != nil {
		g.status = err.Error()
	} else {
		g.status = success
	}
}
func (g *editor) dirty() bool {
	return g.projectDirty || (g.drag != "" && g.dragMoved) || (g.active >= 0 && g.input.Dirty())
}

// The component owns the draft; applying it is a project transaction. A failed
// validation restores the model while retaining the draft and cursor for repair.
func (g *editor) commit() bool {
	if g.active < 0 {
		return true
	}
	f := g.fields[g.active]
	value := g.input.Text()
	if value == f.Value {
		g.active = -1
		return true
	}
	before := g.s.Project.Clone()
	if err := f.set(value); err != nil {
		g.report(err, "")
		return false
	}
	if err := g.s.validateLayout(); err != nil {
		index, draft := g.active, g.input.Clone()
		g.s.Project = before
		g.rebuild()
		g.focus(index)
		g.input = draft
		g.report(err, "")
		return false
	}
	g.s.undo = append(g.s.undo, before)
	if len(g.s.undo) > 100 {
		g.s.undo = g.s.undo[1:]
	}
	g.s.redo = nil
	g.rebuild()
	g.report(nil, "Unsaved changes")
	return true
}
func (g *editor) save() bool {
	if !g.commit() {
		return false
	}
	err := g.s.Save()
	g.projectDirty = g.s.Dirty()
	g.report(err, "Saved "+g.s.Path)
	return err == nil
}
func (g *editor) validate() {
	if !g.commit() {
		return
	}
	_, err := g.s.Project.Resolve()
	g.report(err, "Build inputs are valid. No files were built, signed or submitted.")
}
func (g *editor) settingsPanel() comp.Panel {
	width := min(696, g.w-364)
	title := tabNames[g.tab] + " settings"
	if g.tab == 1 {
		width = 364
		title = "Layout settings"
	}
	return comp.Panel{Bounds: comp.Box(40, 195, width-16, g.h-301), Title: title}
}
func (g *editor) formArea() image.Rectangle { return g.settingsPanel().Content() }
func (g *editor) syncForm() {
	specs := make([]comp.InputSpec, len(g.fields))
	for i, f := range g.fields {
		specs[i] = f.InputSpec
	}
	g.form.SetBounds(g.formArea())
	g.form.SetInputs(specs)
}
func (g *editor) focus(i int) {
	if len(g.fields) == 0 {
		g.active = -1
		return
	}
	i = max(0, min(i, len(g.fields)-1))
	g.active = i
	g.input = comp.NewInput(g.fields[i].InputSpec)
	g.form.Reveal(i)
}
func (g *editor) editInput() {
	result := g.input.Handle(comp.CaptureKeyboard(), comp.SystemClipboard{})
	if result.Err != nil {
		g.report(result.Err, "")
		return
	}
	switch result.Intent {
	case comp.InputCancel:
		g.active = -1
	case comp.InputSubmit:
		g.commit()
	case comp.InputNext, comp.InputPrevious:
		index := g.active
		if !g.commit() || len(g.fields) == 0 {
			return
		}
		delta := 1
		if result.Intent == comp.InputPrevious {
			delta = -1
		}
		g.focus((index + delta + len(g.fields)) % len(g.fields))
	}
}
func (g *editor) switchTab(index int) {
	if index < 0 || index >= len(tabNames) || !g.commit() {
		return
	}
	g.tab = index
	g.form.ScrollTo(0)
	g.selected = ""
	g.rebuild()
}

func (g *editor) Update() error {
	if g.quit {
		return ebiten.Termination
	}
	if err := g.ctx.Err(); err != nil {
		return err
	}
	if ebiten.IsWindowBeingClosed() {
		if g.dirty() {
			g.confirmClose = true
		} else {
			return ebiten.Termination
		}
	}
	mx, my := comp.PointerPosition(g.w, g.h)
	mouse := image.Pt(mx, my)
	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	if click {
		mx, my = comp.PointerPressPosition(g.w, g.h)
		mouse = image.Pt(mx, my)
	}
	if g.closeDialog().Handle(mouse, click, inpututil.IsKeyJustPressed(ebiten.KeyEscape)) {
		return nil
	}
	if comp.CommandKey() {
		if inpututil.IsKeyJustPressed(ebiten.KeyS) {
			g.save()
			return nil
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyZ) {
			if g.active >= 0 {
				g.rebuild()
			} else {
				if ebiten.IsKeyPressed(ebiten.KeyShift) {
					g.s.Redo()
				} else {
					g.s.Undo()
				}
				g.rebuild()
			}
			return nil
		}
		for i, key := range []ebiten.Key{ebiten.KeyDigit1, ebiten.KeyDigit2, ebiten.KeyDigit3, ebiten.KeyDigit4, ebiten.KeyDigit5, ebiten.KeyDigit6} {
			if inpututil.IsKeyJustPressed(key) {
				g.switchTab(i)
				return nil
			}
		}
	}
	if g.active >= 0 {
		g.editInput()
	} else if inpututil.IsKeyJustPressed(ebiten.KeyTab) && len(g.fields) > 0 {
		g.focus(0)
	} else {
		g.nudgeSelected()
	}
	if click {
		if g.tabs().Click(mouse) || (g.tab > 0 && g.stepToggle().Click(mouse)) || comp.ClickButtons(g.controls(), mouse) {
			return nil
		}
		if i, ok := g.form.Hit(mouse); ok {
			if g.active != i {
				if !g.commit() {
					return nil
				}
				g.focus(i)
			}
			if g.active >= 0 && len(g.input.Spec.Choices) > 0 {
				g.cycleChoice(g.active)
			}
			return nil
		}
		if !g.commit() {
			return nil
		}
		if g.tab == 1 && g.enabled() {
			t := g.transform()
			if mouse.In(g.previewArea()) && mouse.In(t.bounds) && !(g.previewActual && ebiten.IsKeyPressed(ebiten.KeySpace)) {
				_, _, size, _, items := layout(g.s.Project.DMG, g.s.Project.App)
				x, y := t.content(float64(mx), float64(my))
				g.selected = ""
				for i := len(items) - 1; i >= 0; i-- {
					item := items[i]
					if math.Abs(x-float64(item.X)) <= float64(size)/2 && math.Abs(y-float64(item.Y)) <= float64(size)/2 {
						g.selected = item.Path
						g.drag = item.Path
						g.dragX = x - float64(item.X)
						g.dragY = y - float64(item.Y)
						g.dragMoved = false
						break
					}
				}
				g.rebuild()
				g.form.ScrollTo(0)
			}
		}
		if g.tab == 1 && g.enabled() && g.previewActual && mouse.In(g.previewArea()) && g.drag == "" {
			g.panning = true
			g.panStartX, g.panStartY = mx, my
			g.panOriginX, g.panOriginY = g.panX, g.panY
		}
	}
	if g.panning {
		px, py := comp.PointerPosition(g.w, g.h)
		g.panX, g.panY = g.panOriginX+px-g.panStartX, g.panOriginY+py-g.panStartY
		g.clampPan()
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			g.panning = false
		}
	}
	if g.drag != "" {
		// Apply the release position too: a short drag can finish between ticks.
		px, py := comp.PointerPosition(g.w, g.h)
		x, y := g.transform().content(float64(px), float64(py))
		x -= g.dragX
		y -= g.dragY
		_, _, _, _, items := layout(g.s.Project.DMG, g.s.Project.App)
		for _, i := range items {
			if i.Path == g.drag && (int(math.Round(x)) != i.X || int(math.Round(y)) != i.Y) {
				if !g.dragMoved {
					g.s.checkpoint()
					g.dragMoved = true
				}
				g.s.move(g.drag, int(math.Round(x)), int(math.Round(y)))
				break
			}
		}
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			g.drag = ""
			if g.dragMoved {
				g.rebuild()
				g.report(nil, "Position updated. Arrow keys nudge; Shift moves 10 pixels.")
			}
		}
	}

	if mouse.In(g.form.Bounds) {
		_, wheel := ebiten.Wheel()
		g.form.ScrollBy(-int(wheel * 38))
	}
	return nil
}
