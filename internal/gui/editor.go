package gui

import (
	"context"
	"image"

	"github.com/ironpark/ggfx"

	"github.com/ironpark/zapp"
)

// editor is the project being edited and everything the GUI shows of it.
// Its state is grouped by what owns it; the groups are embedded, so their
// fields read as the editor's own.
type editor struct {
	ctx     context.Context
	s       *Session
	desktop *desktopModel
	w, h    int // the window's starting size

	pageState
	formState
	designerState
	assetState
	statusState
	signing signingAssist
	build   *buildJob

	confirmClose, quit bool
	// wake schedules the UI thread to apply finished background work and act
	// on quit; nil outside a live window.
	wake   func()
	posted chan func()
}

// pageState is which page is shown and how: the tab, the PKG component,
// raw or form views, and the methods chosen on the Signing and
// Notarization tabs.
type pageState struct {
	tab, componentIndex        int
	helpOpen                   bool
	dmgYAML, pkgRaw, depRaw    bool
	dmgAdvanced, pkgAdvanced   bool
	signMode, notaryMode       int
	signModeSet, notaryModeSet bool
	signStash                  zapp.SignConfig     // the other sign methods' settings
	notaryStash                zapp.NotarizeConfig // the other notary methods' settings
	disabled                   zapp.Project        // the settings of sections switched off
}

// formState is the fields of the page and the one being edited, whose
// draft is kept apart from the project until it commits.
type formState struct {
	fields         []field
	inspectorStart int // fields from here on are the selected DMG item's
	active         int // the field being edited, or -1
	input          Input
	liveBase       *zapp.Project // the project before a draft was previewed in it
	projectDirty   bool
}

// designerState is the DMG preview canvas: the selected item, a drag in
// progress and the actual-size view's pan.
type designerState struct {
	previewBounds          image.Rectangle
	selected               string
	drag                   string
	dragX, dragY           float64
	dragMoved              bool
	previewActual, panning bool
	// pan is the actual-size view offset; panStart and panOrigin capture where
	// the current drag began.
	pan, panStart, panOrigin image.Point
}

// assetState is the images the GUI draws and what it knows of the files
// behind them.
type assetState struct {
	assets          map[string]*ggfx.Image
	appIconPending  map[string]bool
	appIconPaths    map[string]string
	itemKinds       map[string]string
	projectIconPath string // app whose icon projectIconKey holds
	previewError    string
	previewSig      string
}

// statusState is what the status line, the health badge and Go to issue
// report.
type statusState struct {
	status   string
	failed   bool
	issue    *validationIssue
	health   projectHealth
	healthOf *zapp.Project // the project health describes
}

// newEditor opens s in an editor whose window starts w by h.
func newEditor(ctx context.Context, s *Session, w, h int) *editor {
	return &editor{
		ctx: ctx, s: s, w: w, h: h,
		formState:   formState{active: -1},
		assetState:  assetState{assets: map[string]*ggfx.Image{}},
		statusState: statusState{status: "Edit settings, then Save. Validation checks build inputs without building."},
	}
}

func Run(ctx context.Context, s *Session) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	return newEditor(ctx, s, 1200, 840).runWidgets()
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
	g.input = NewInput(g.fields[i].InputSpec)
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
