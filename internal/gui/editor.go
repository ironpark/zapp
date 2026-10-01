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
	componentIndex                        int
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
	inspectorStart                           int
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
	painter, err := comp.NewPainter(ttf)
	if err != nil && ttf != nil {
		painter, err = comp.NewPainter(nil)
	}
	if err != nil {
		return err
	}
	defer painter.Close()
	g := &editor{ctx: ctx, s: s, w: 1200, h: 840, active: -1, ui: painter, assets: map[string]*ggfx.Image{}, status: "Edit settings, then Save. Validation checks build inputs without building."}
	defer func() {
		if g.previewSurface != nil {
			g.previewSurface.Deallocate()
		}
	}()
	return g.runWidgets()
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

func (g *editor) switchTab(index int) {
	if index < 0 || index >= len(sections) || !g.commit() {
		return
	}
	g.tab = index
	g.selected = ""
	g.rebuild()
}
