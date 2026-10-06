package gui

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/ironpark/ggui"
	guiruntime "github.com/ironpark/ggui/runtime"
)

// desktopModel is what the window shows of the editor, and the window's own
// state. The editor is the one source of truth, held in a ggui.Store: every
// view of it below is selected from it, recomputed when the store publishes
// a change, so none is a copy to keep in step. A field's text and error are
// the editor's own reactive state, which the inputs bind to directly.
type desktopModel struct {
	editor  *editor
	store   *ggui.Store[*editor]
	dialogs guiruntime.FilePicker

	Workspace                                   *ggui.DerivedValue[workspaceState]
	Fields, Inspector                           *ggui.DerivedValue[[]*fieldState]
	Components                                  *ggui.DerivedValue[[]componentRow]
	Items                                       *ggui.DerivedValue[[]layoutItem]
	Status, SaveState, Title                    *ggui.DerivedValue[string]
	Health                                      *ggui.DerivedValue[projectHealth]
	Enabled                                     *ggui.DerivedValue[[tabCount]bool]
	Failed, CanUndo, CanRedo, Busy, Help, Close *ggui.DerivedValue[bool]
	Tab                                         *ggui.DerivedValue[int]
	Modal                                       *ggui.DerivedValue[modalState]
	// Apart from Workspace, so that they change without rebuilding the page:
	// the tab with an issue, why part of the DMG preview could not be drawn,
	// and the Signing tab's keychain identities and check.
	IssueTab     *ggui.DerivedValue[int]
	PreviewError *ggui.DerivedValue[string]
	Signing      *ggui.DerivedValue[signingState]

	// The window's own.
	Dark                  *ggui.StateValue[bool]
	Split, InspectorSplit *ggui.StateValue[float64]
	DropHover             *ggui.StateValue[bool]
	scroll                map[string]*ggui.StateValue[float64]
}

type workspaceState struct {
	Tab, Component, SignMethod, NotaryMethod            int
	Enabled, Full, Raw, Advanced, Actual, DefaultLayout bool
	Selected                                            string
}

// signingState is the Signing tab's keychain identities offered
// (newline-separated) and the credential check for the current settings.
type signingState struct {
	Identities                          string
	IdentitiesListed, IdentitiesListing bool
	Check                               signCheck
}

type componentRow struct {
	Index int
	Label string
}

type modalState struct {
	Title, Message              string
	Log                         string // the build's lines so far
	Reveal                      string // artifact to show in the file manager
	Finished, Cancelling, Issue bool
}

// allowClose decides a close request from the window's button or the
// platform quit command. A running build is cancelled first and the decision
// waits until it has stopped (pollBuild calls finishClose); unsaved edits open
// the confirm dialog, whose buttons end the app through requestQuit.
func (m *desktopModel) allowClose() bool {
	g := m.editor
	if g.build != nil {
		g.build.closeRequested = true
		if g.build.finished {
			g.finishClose()
		} else if !g.build.cancelling {
			g.dismissBuild()
		}
		g.invalidate()
		return false
	}
	if g.dirty() {
		g.confirmClose = true
		g.invalidate()
		return false
	}
	return true
}

func newDesktopModel(g *editor) *desktopModel {
	m := &desktopModel{
		editor:         g,
		store:          ggui.NewStore(g),
		Dark:           ggui.State(true),
		Split:          ggui.State(0.3),
		InspectorSplit: ggui.State(0.7),
		DropHover:      ggui.State(false),
		scroll:         map[string]*ggui.StateValue[float64]{},
	}
	m.Workspace = derive(m, g.workspace)
	m.IssueTab = derive(m, func() int {
		if g.issue == nil {
			return -1
		}
		return g.issue.tab
	})
	m.PreviewError = derive(m, func() string {
		if g.tab != tabDMG {
			return ""
		}
		return g.previewError
	})
	m.Signing = derive(m, func() signingState {
		if g.tab != tabSign {
			return signingState{}
		}
		return signingState{g.signing.identities, g.signing.listed, g.signing.listing, g.signingCheck()}
	})
	m.Fields = derive(m, func() []*fieldState { return g.fieldStates(0, g.inspectorStart) }).WithEqual(slices.Equal)
	m.Inspector = derive(m, func() []*fieldState { return g.fieldStates(g.inspectorStart, len(g.fields)) }).WithEqual(slices.Equal)
	m.Components = derive(m, g.componentRows).WithEqual(slices.Equal)
	m.Items = derive(m, func() []layoutItem {
		if g.tab != tabDMG || !g.enabled() {
			return []layoutItem{}
		}
		return slices.Clone(g.s.layout().Items)
	}).WithEqual(slices.Equal)
	m.Tab = derive(m, func() int { return g.tab })
	m.Help = derive(m, func() bool { return g.helpOpen })
	m.Close = derive(m, func() bool { return g.confirmClose })
	m.Status = derive(m, func() string { return g.status })
	m.Failed = derive(m, func() bool { return g.failed })
	m.CanUndo = derive(m, g.s.CanUndo)
	m.CanRedo = derive(m, g.s.CanRedo)
	m.Busy = derive(m, func() bool { return g.build != nil })
	m.SaveState = derive(m, g.saveState)
	m.Health = derive(m, func() projectHealth { return g.health })
	m.Enabled = derive(m, func() [tabCount]bool {
		var enabled [tabCount]bool
		for i, s := range sections {
			enabled[i] = s.Enabled(g.s.Project)
		}
		return enabled
	})
	m.Title = derive(m, func() string { return cmp.Or(g.health.App.Name, "Untitled project") })
	m.Modal = derive(m, g.modal)
	g.desktop = m
	return m
}

// derive is fn as a value the window reads, selected from the editor's
// store, so it is recomputed whenever the editor changes.
func derive[T any](m *desktopModel, fn func() T) *ggui.DerivedValue[T] {
	return ggui.Select(m.store, func(*editor) T { return fn() })
}

// workspace is what the page under the tabs is built from.
func (g *editor) workspace() workspaceState {
	v := workspaceState{Tab: g.tab, Component: g.componentIndex, Enabled: g.enabled(), Selected: g.selected, Actual: g.previewActual}
	switch g.tab {
	case tabDMG:
		v.Raw, v.Advanced = g.dmgYAML, g.dmgAdvanced
		if g.s.Project.DMG != nil {
			v.DefaultLayout = g.s.Project.DMG.Contents == nil
		}
	case tabPKG:
		v.Raw, v.Advanced = g.pkgRaw, g.pkgAdvanced
		v.Full = g.componentListVisible()
	case tabDep:
		v.Raw = g.depRaw
	case tabSign:
		v.SignMethod = g.signMethod()
	case tabNotarize:
		v.NotaryMethod = g.notaryMethod()
	}
	return v
}

// fieldStates is the states of the page's fields from i to j.
func (g *editor) fieldStates(i, j int) []*fieldState {
	states := make([]*fieldState, 0, j-i)
	for _, f := range g.fields[i:j] {
		states = append(states, f.state)
	}
	return states
}

// componentRows lists the PKG components while the component list shows.
func (g *editor) componentRows() []componentRow {
	rows := []componentRow{}
	if !g.componentListVisible() {
		return rows
	}
	for i, c := range g.s.Project.PKG.Components {
		rows = append(rows, componentRow{i, cmp.Or(c.ID, fmt.Sprintf("Component %d", i+1))})
	}
	return rows
}

// saveState is the toolbar's word on the project's file.
func (g *editor) saveState() string {
	switch {
	case g.dirty():
		return "Unsaved changes"
	case !g.s.exists:
		return "New project"
	}
	return "Saved"
}

// modal is the build dialog, while a build runs or until it is dismissed.
func (g *editor) modal() modalState {
	j := g.build
	if j == nil {
		return modalState{}
	}
	title, message := g.buildDialog()
	v := modalState{Title: title, Message: message, Log: j.text(), Finished: j.finished, Cancelling: j.cancelling, Issue: g.issue != nil}
	if j.finished && j.err == nil {
		v.Reveal = cmp.Or(j.artifacts.DMG, j.artifacts.PKG, j.artifacts.App)
	}
	return v
}

// invalidate publishes a change to the editor, so what the window selects
// from it is recomputed; inside an action, once the action ends.
func (g *editor) invalidate() {
	if g.desktop != nil {
		g.desktop.store.Changed()
	}
}

// action is fn as a handler: one change to the editor, published once
// however many parts of it report one.
func (g *editor) action(fn func()) func() {
	return func() {
		if g.desktop == nil {
			fn()
			return
		}
		g.desktop.store.Update(func(*editor) { fn() })
	}
}

func (m *desktopModel) offset(key string) *ggui.StateValue[float64] {
	if m.scroll[key] == nil {
		m.scroll[key] = ggui.State(0.0)
	}
	return m.scroll[key]
}

// fieldBinding binds an input to its field's text. An edit makes the field
// the one being edited, committing another first, and previews the draft.
func (m *desktopModel) fieldBinding(f *fieldState) ggui.Binding[string] {
	return ggui.Bind(f.Text.Get, func(value string) {
		g := m.editor
		if !g.matchesField(f.ID) {
			return
		}
		if g.active != f.ID.index {
			if !g.commit() {
				return
			}
			g.focus(f.ID.index)
		}
		g.input.SetText(value)
		g.clearFieldError()
		g.previewInput()
	})
}

func (m *desktopModel) commitField(f *fieldState) {
	g := m.editor
	if g.matchesField(f.ID) && g.active == f.ID.index {
		if !g.commit() {
			f.focus.Focus()
		}
	}
	g.invalidate()
}

func (m *desktopModel) selectTab(i int) {
	if i != m.editor.tab {
		m.editor.switchTab(i)
	}
}

func (m *desktopModel) source(raw bool) { m.editor.setRaw(raw) }

func (m *desktopModel) advanced() { m.editor.toggleAdvanced() }

// setRaw switches the current tab between its form and its raw text view. An
// invalid draft blocks the switch, so it reports whether the view changed.
func (g *editor) setRaw(raw bool) bool {
	if !g.commit() {
		return false
	}
	switch g.tab {
	case tabDMG:
		g.dmgYAML = raw
	case tabPKG:
		g.pkgRaw = raw
	case tabDep:
		g.depRaw = raw
	}
	g.rebuild()
	return true
}

// toggleAdvanced shows or hides the current tab's advanced fields.
func (g *editor) toggleAdvanced() bool {
	if !g.commit() {
		return false
	}
	if g.tab == tabDMG {
		g.dmgAdvanced = !g.dmgAdvanced
	} else {
		g.pkgAdvanced = !g.pkgAdvanced
	}
	g.rebuild()
	return true
}

func (m *desktopModel) selectComponent(i int) {
	g := m.editor
	if !g.commit() {
		return
	}
	g.componentIndex = i
	g.pkgRaw = false
	g.rebuild()
}

// assetsChanged reports that an image arrived or went.
func (g *editor) assetsChanged() { g.invalidate() }
