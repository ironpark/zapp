package gui

import (
	"context"
	"image"
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
	inspector                                comp.Form
	inspectorStart, itemScroll               int
	picking                                  <-chan pickResult
	ui                                       *comp.Painter
	status                                   string
	failed, confirmClose, quit, projectDirty bool
	selected                                 string
	drag                                     string
	dragX, dragY                             float64
	dragMoved                                bool
	assets                                   map[string]*ebiten.Image
	itemKinds                                map[string]string
	previewError                             string
	previewSig                               string
	dmgAdvanced                              bool
	choiceOpen                               bool
	choiceIndex                              int
	previewActual, panning                   bool
	panX, panY, panStartX, panStartY         int
	panOriginX, panOriginY                   int
}

// systemFontPaths are probed in order for a Unicode-capable UI font. Absent
// paths simply fail to open, so the list stays cross-platform.
var systemFontPaths = []string{
	"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
	"C:/Windows/Fonts/malgun.ttf",
	"/usr/share/fonts/truetype/nanum/NanumGothic.ttf",
}

func Run(ctx context.Context, s *Session) error {
	// Prefer a local Unicode font when available, with the bundled font as
	// fallback. Only the chosen candidate is read and parsed; Arial Unicode
	// alone is ~20 MB, so parsing every candidate would be wasteful.
	var ttf []byte
	for _, name := range systemFontPaths {
		if data, e := os.ReadFile(name); e == nil {
			ttf = data
			break
		}
	}
	painter, err := comp.NewPainter(ttf, comp.DarkTheme())
	if err != nil && ttf != nil {
		painter, err = comp.NewPainter(nil, comp.DarkTheme())
	}
	if err != nil {
		return err
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
	g.inspector.SetBounds(g.inspectorPanel().Content())
	if g.choiceOpen {
		g.revealField(g.active)
	}
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
		g.fieldError(err)
		return false
	}
	if err := g.s.validateLayout(); err != nil {
		index, draft := g.active, g.input.Clone()
		g.s.Project = before
		g.rebuild()
		g.focus(index)
		g.input = draft
		g.fieldError(err)
		return false
	}
	g.s.push(before)
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
	if !g.validatePaths() {
		return
	}
	_, err := g.s.Project.Resolve()
	if err != nil {
		g.locateValidationError(err)
	}
	g.report(err, "Build inputs are valid. No files were built, signed or submitted.")
}

const footerHeight = 40

func (g *editor) contentBottom() int { return g.h - footerHeight - 16 }

func (g *editor) settingsPanel() comp.Panel {
	width := min(696, g.w-364)
	title := g.section().Name + " settings"
	if g.tab == tabDMG {
		width = 364
		title = "Layout settings"
	}
	return comp.Panel{Bounds: comp.Box(24, 195, width, g.contentBottom()-195), Title: title}
}
func (g *editor) formArea() image.Rectangle {
	area := g.settingsPanel().Content()
	if g.choiceOpen && g.active >= 0 {
		area.Max.Y -= len(g.input.Spec.Choices)*choiceRowHeight + 8
	}
	return area
}
func (g *editor) syncForm() {
	specs := make([]comp.InputSpec, len(g.fields))
	for i, f := range g.fields {
		specs[i] = f.InputSpec
	}
	g.form.SetBounds(g.formArea())
	g.form.SetInputs(specs[:g.inspectorStart])
	g.inspector.SetBounds(g.inspectorPanel().Content())
	g.inspector.SetInputs(specs[g.inspectorStart:])
}
func (g *editor) focus(i int) {
	if len(g.fields) == 0 {
		g.active = -1
		return
	}
	i = max(0, min(i, len(g.fields)-1))
	g.active = i
	g.input = comp.NewInput(g.fields[i].InputSpec)
	g.revealField(i)
}
func (g *editor) editInput() {
	before := g.input.Text()
	result := g.input.Handle(comp.CaptureKeyboard(), comp.SystemClipboard{})
	if before != g.input.Text() {
		g.clearFieldError()
	}
	if result.Err != nil {
		g.report(result.Err, "")
		return
	}
	switch result.Intent {
	case comp.InputOpenChoice:
		g.openChoice()
	case comp.InputCancel:
		g.clearFieldError()
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
	if index < 0 || index >= len(sections) || !g.commit() {
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
	if g.picking != nil {
		g.pollPicker()
		return nil
	}
	if ebiten.IsWindowBeingClosed() {
		if !g.dirty() {
			return ebiten.Termination
		}
		g.confirmClose = true
	}
	in := captureTick(g.w, g.h)
	if g.confirmClose && g.closeDialog().Handle(in.mouse, in.click, inpututil.IsKeyJustPressed(ebiten.KeyEscape)) {
		return nil
	}
	if g.choiceOpen {
		g.handleChoice(in, comp.CaptureKeyboard())
		return nil
	}
	if files := ebiten.DroppedFiles(); files != nil {
		x, y := ebiten.CursorPosition()
		g.dropFiles(files, image.Pt(x, y))
		return nil
	}
	if g.handleShortcuts() {
		return nil
	}
	g.handleTyping()
	if in.click && g.handleClick(in) {
		return nil
	}
	g.updatePan(in)
	g.updateDrag(in)
	g.scrollForm(in)
	return nil
}
