package gui

import (
	"context"
	"image"
	"os"

	"github.com/ironpark/ggfx"

	"github.com/ironpark/zapp"
	"github.com/ironpark/zapp/internal/gui/comp"
)

type editor struct {
	previewBounds                         image.Rectangle
	desktop                               *desktopModel
	previewSurface                        *ggfx.Image
	helpOpen, pkgAdvanced, pkgRaw, depRaw bool
	componentIndex, componentScroll       int
	signMode, notaryMode                  int
	signModeSet, notaryModeSet            bool
	signStash                             zapp.SignConfig
	notaryStash                           zapp.NotarizeConfig
	issue                                 *validationIssue
	health                                projectHealth
	signing                               signingAssist
	projectIconPath                       string        // app whose icon projectIconKey holds
	healthOf                              *zapp.Project // the project health describes

	dmgYAML                                  bool
	appIconResults                           chan appIconResult
	appIconPending                           map[string]bool
	appIconPaths                             map[string]string
	build                                    *buildJob
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
	// wake schedules the UI thread to apply finished background work and act
	// on quit; nil outside a live window.
	wake                   func()
	posted                 chan func()
	selected               string
	drag                   string
	dragX, dragY           float64
	dragMoved              bool
	assets                 map[string]*ggfx.Image
	itemKinds              map[string]string
	previewError           string
	previewSig             string
	liveBase               *zapp.Project
	dmgAdvanced            bool
	previewActual, panning bool
	// pan is the actual-size view offset; panStart and panOrigin capture where
	// the current drag began.
	pan, panStart, panOrigin image.Point
}

// systemFontPaths are probed in order for a Unicode-capable UI font. Absent
// paths simply fail to open, so the list stays cross-platform.
var systemFontPaths = []string{
	"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
	"C:/Windows/Fonts/malgun.ttf",
	"/usr/share/fonts/truetype/nanum/NanumGothic.ttf",
}

func Run(ctx context.Context, s *Session) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
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
	g := &editor{ctx: ctx, s: s, w: 1200, h: 840, active: -1, ui: painter, assets: map[string]*ggfx.Image{}, status: "Edit settings, then Save. Validation checks build inputs without building."}
	g.rebuild()
	defer func() {
		if g.previewSurface != nil {
			g.previewSurface.Deallocate()
		}
	}()
	return g.runWidgets()
}
func (g *editor) Layout(w, h int) (int, int) {
	g.w = w
	g.h = h
	g.form.SetBounds(g.formArea())
	if g.tab == tabDMG && g.dmgYAML && len(g.form.Inputs) > 0 {
		g.form.Inputs[0].Height = g.yamlHeight()
		g.fields[0].Height = g.yamlHeight()
		if g.active == 0 {
			g.input.Spec.Height = g.yamlHeight()
		}
	}
	g.inspector.SetBounds(g.inspectorArea())
	return w, h
}

// wakeFunc returns the hook that asks the UI thread to apply finished
// background work, or a no-op outside a live window. A goroutine captures it
// before it starts rather than reading the field concurrently.
func (g *editor) wakeFunc() func() {
	if g.wake != nil {
		return g.wake
	}
	return func() {}
}

// poster returns, on the UI thread, the function a goroutine hands finished
// work to; the next pump applies it on the UI thread.
func (g *editor) poster() func(func()) {
	if g.posted == nil {
		g.posted = make(chan func(), 8)
	}
	posted, wake := g.posted, g.wakeFunc()
	return func(fn func()) {
		posted <- fn
		wake()
	}
}

// pollPosted applies work goroutines handed over through poster.
func (g *editor) pollPosted() {
	for {
		select {
		case fn := <-g.posted:
			fn()
		default:
			return
		}
	}
}

// requestQuit ends the app at the next frame, and makes sure there is one.
func (g *editor) requestQuit() {
	g.quit = true
	if g.wake != nil {
		g.wake()
	}
}
func (g *editor) report(err error, success string) {
	defer g.invalidate()
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
	if g.liveBase != nil {
		index, draft := g.active, g.input.Clone()
		g.rebuild()
		g.active, g.input = index, draft
	}
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
	if g.issueOn(g.tab) && g.issue.label == f.Label {
		g.issue = nil
	}
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
	g.issue = nil
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
	if err == nil {
		g.rebuild()
	}
	g.recheckHealth()
	g.report(err, "Build inputs are valid. No files were built, signed or submitted.")
}

const footerHeight = 28
const workspaceTop = 128

func (g *editor) contentBottom() int { return g.h - footerHeight - 16 }

func (g *editor) settingsPanel() comp.Panel {
	x := 24
	width := g.w - 48
	if g.helpOpen {
		width = min(760, g.w-384)
	}
	if g.componentListVisible() {
		x = 240
		width = g.w - x - 24
		if g.helpOpen {
			width = min(760, g.w-x-360)
		}
	}
	title := g.section().Name + " settings"
	if g.tab == tabDMG {
		x = 24
		width = min(364, max(320, g.w/3-56))
		title = "Layout settings"
	}
	panel := comp.Panel{Bounds: comp.Box(x, workspaceTop, width, g.contentBottom()-workspaceTop), Title: title}
	if g.tab == tabDMG {
		panel.TitleInset = 132
	} else {
		panel.TitleInset = 80
		if g.tab == tabDep || g.componentListVisible() || (g.tab == tabPKG && g.pkgRaw) {
			panel.TitleInset = 220
		}
	}
	return panel
}
func (g *editor) formArea() image.Rectangle {
	area := g.settingsPanel().Content()
	if g.tab == tabSign || g.tab == tabNotarize {
		area.Min.Y += 48
	}
	if g.tab == tabPKG || (g.tab == tabDep && !g.depRaw) {
		area.Max.Y -= 44
	}
	if g.tab == tabDMG && !g.dmgYAML {
		area.Max.Y -= 44
	}
	return area
}
func (g *editor) syncForm() {
	if g.desktop != nil {
		return
	}
	specs := make([]comp.InputSpec, len(g.fields))
	for i, f := range g.fields {
		specs[i] = f.InputSpec
	}
	g.form.SetBounds(g.formArea())
	g.form.SetInputs(specs[:g.inspectorStart])
	g.inspector.SetBounds(g.inspectorArea())
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
	if g.desktop != nil {
		id := g.fieldIdentity(i)
		g.desktop.focus = &id
	}
	g.revealField(i)
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
