package gui

import (
	"cmp"
	"fmt"
	"reflect"
	"slices"

	"github.com/ironpark/ggui"
	guiruntime "github.com/ironpark/ggui/runtime"
)

// desktopModel separates reactive presentation state from project transactions.
// Values/errors update individual controls; structural changes rebuild only the
// workspace. No frame counter or absolute form geometry drives the widget tree.
type desktopModel struct {
	Assets                                      *ggui.StateValue[int]
	editor                                      *editor
	dialogs                                     guiruntime.FilePicker
	Workspace                                   *ggui.StateValue[workspaceState]
	Fields, Inspector                           *ggui.StateValue[[]*desktopField]
	Components                                  *ggui.StateValue[[]componentRow]
	Items                                       *ggui.StateValue[[]layoutItem]
	Status, SaveState, Title                    *ggui.StateValue[string]
	Health                                      *ggui.StateValue[projectHealth]
	Enabled                                     *ggui.StateValue[[tabCount]bool]
	Failed, CanUndo, CanRedo, Busy, Help, Close *ggui.StateValue[bool]
	Tab                                         *ggui.StateValue[int]
	Modal                                       *ggui.StateValue[modalState]
	Dark                                        *ggui.StateValue[bool]
	Split, InspectorSplit                       *ggui.StateValue[float64]
	DropHover                                   *ggui.StateValue[bool]
	// Kept apart from Workspace, so that they change without rebuilding
	// the page: the tab with an issue, why part of the DMG preview could not
	// be drawn, and the Signing tab's keychain identities and check.
	IssueTab     *ggui.StateValue[int]
	PreviewError *ggui.StateValue[string]
	Signing      *ggui.StateValue[signingState]
	cache        map[fieldIdentity]*desktopField
	scroll       map[string]*ggui.StateValue[float64]
	// focus holds each field's FocusRef by identity, so a field can be
	// focused before sync builds it, as revealField does after a tab switch.
	focus map[fieldIdentity]*ggui.FocusRef
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
type fieldIdentity struct {
	tab, component, index int
	item, label, syntax   string
}
type desktopField struct {
	ID           fieldIdentity
	Spec         InputSpec
	Value, Error *ggui.StateValue[string]
	focus        *ggui.FocusRef // focuses the field's input
}

func (g *editor) fieldIdentity(i int) fieldIdentity {
	f := g.fields[i]
	return fieldIdentity{g.tab, g.componentIndex, i, g.selected, f.Label, f.Syntax}
}
func (g *editor) matchesField(id fieldIdentity) bool {
	return id.index >= 0 && id.index < len(g.fields) && g.fieldIdentity(id.index) == id
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
		m.sync()
		return false
	}
	if g.dirty() {
		g.confirmClose = true
		m.sync()
		return false
	}
	return true
}

func newDesktopModel(g *editor) *desktopModel {
	m := &desktopModel{
		editor:         g,
		Assets:         ggui.State(0),
		Workspace:      ggui.State(workspaceState{}),
		Fields:         ggui.State([]*desktopField{}).WithEqual(slices.Equal),
		Inspector:      ggui.State([]*desktopField{}).WithEqual(slices.Equal),
		Components:     ggui.State([]componentRow{}).WithEqual(slices.Equal),
		Items:          ggui.State([]layoutItem{}).WithEqual(slices.Equal),
		Status:         ggui.State(""),
		SaveState:      ggui.State(""),
		Title:          ggui.State(""),
		Health:         ggui.State(projectHealth{}),
		Enabled:        ggui.State([tabCount]bool{}),
		Failed:         ggui.State(false),
		CanUndo:        ggui.State(false),
		CanRedo:        ggui.State(false),
		Busy:           ggui.State(false),
		Help:           ggui.State(g.helpOpen),
		Close:          ggui.State(false),
		Tab:            ggui.State(g.tab),
		Modal:          ggui.State(modalState{}),
		Dark:           ggui.State(true),
		Split:          ggui.State(0.3),
		InspectorSplit: ggui.State(0.7),
		DropHover:      ggui.State(false),
		IssueTab:       ggui.State(-1),
		PreviewError:   ggui.State(""),
		Signing:        ggui.State(signingState{}),
		cache:          map[fieldIdentity]*desktopField{},
		scroll:         map[string]*ggui.StateValue[float64]{},
		focus:          map[fieldIdentity]*ggui.FocusRef{},
	}
	g.desktop = m
	m.sync()
	return m
}
func (g *editor) invalidate() {
	if g.desktop != nil {
		g.desktop.sync()
	}
}
func (g *editor) action(fn func()) func() {
	return func() {
		if fn != nil {
			fn()
		}
		g.invalidate()
	}
}
func (m *desktopModel) sync() {
	g := m.editor
	v := workspaceState{Tab: g.tab, Component: g.componentIndex, Enabled: g.enabled(), Selected: g.selected, Actual: g.previewActual}
	issueTab, previewError, signing := -1, "", signingState{}
	if g.issue != nil {
		issueTab = g.issue.tab
	}
	switch g.tab {
	case tabDMG:
		v.Raw, v.Advanced = g.dmgYAML, g.dmgAdvanced
		previewError = g.previewError
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
		signing = signingState{g.signing.identities, g.signing.listed, g.signing.listing, g.signingCheck()}
	case tabNotarize:
		v.NotaryMethod = g.notaryMethod()
	}
	m.Workspace.Set(v)
	m.IssueTab.Set(issueTab)
	m.PreviewError.Set(previewError)
	m.Signing.Set(signing)
	m.Tab.Set(g.tab)
	m.Help.Set(g.helpOpen)
	m.Close.Set(g.confirmClose)
	m.Status.Set(g.status)
	m.Failed.Set(g.failed)
	m.CanUndo.Set(g.s.CanUndo())
	m.CanRedo.Set(g.s.CanRedo())
	m.Busy.Set(g.build != nil)
	state := "Saved"
	if !g.s.exists {
		state = "New project"
	}
	if g.dirty() {
		state = "Unsaved changes"
	}
	m.SaveState.Set(state)
	m.Health.Set(g.health)
	var enabled [tabCount]bool
	for i, s := range sections {
		enabled[i] = s.Enabled(g.s.Project)
	}
	m.Enabled.Set(enabled)
	title := g.health.App.Name
	if title == "" {
		title = "Untitled project"
	}
	m.Title.Set(title)
	fields, inspector := []*desktopField{}, []*desktopField{}
	next := map[fieldIdentity]*desktopField{}
	for i, f := range g.fields {
		id := g.fieldIdentity(i)
		spec := f.InputSpec
		spec.Value = ""
		spec.Error = ""
		current := m.cache[id]
		if current == nil || !reflect.DeepEqual(current.Spec, spec) {
			current = &desktopField{ID: id, Spec: spec, Value: ggui.State(f.Value), Error: ggui.State(f.Error), focus: m.fieldFocus(id)}
		}
		value := f.Value
		if g.active == i {
			value = g.input.Text()
		}
		current.Value.Set(value)
		current.Error.Set(f.Error)
		next[id] = current
		if i < g.inspectorStart {
			fields = append(fields, current)
		} else {
			inspector = append(inspector, current)
		}
	}
	m.cache = next
	for id := range m.focus {
		if next[id] == nil {
			delete(m.focus, id)
		}
	}
	m.Fields.Set(fields)
	m.Inspector.Set(inspector)
	components := []componentRow{}
	if v.Full {
		for i, c := range g.s.Project.PKG.Components {
			label := c.ID
			if label == "" {
				label = fmt.Sprintf("Component %d", i+1)
			}
			components = append(components, componentRow{i, label})
		}
	}
	m.Components.Set(components)
	items := []layoutItem{}
	if g.tab == tabDMG && g.enabled() {
		items = append(items, g.s.layout().Items...)
	}
	m.Items.Set(items)
	modal := modalState{}
	if g.build != nil {
		j := g.build
		title, message := g.buildDialog()
		modal = modalState{Title: title, Message: message, Log: j.text(), Finished: j.finished, Cancelling: j.cancelling, Issue: g.issue != nil}
		if j.finished && j.err == nil {
			modal.Reveal = cmp.Or(j.artifacts.DMG, j.artifacts.PKG, j.artifacts.App)
		}
	}
	m.Modal.Set(modal)
}

// fieldFocus is the FocusRef of the field with identity id.
func (m *desktopModel) fieldFocus(id fieldIdentity) *ggui.FocusRef {
	if m.focus[id] == nil {
		m.focus[id] = &ggui.FocusRef{}
	}
	return m.focus[id]
}
func (m *desktopModel) offset(key string) *ggui.StateValue[float64] {
	if m.scroll[key] == nil {
		m.scroll[key] = ggui.State(0.0)
	}
	return m.scroll[key]
}

func (m *desktopModel) fieldBinding(f *desktopField) ggui.Binding[string] {
	return ggui.Bind(f.Value.Get, func(value string) {
		g := m.editor
		if !g.matchesField(f.ID) {
			return
		}
		if g.active != f.ID.index {
			if !g.commit() {
				m.sync()
				return
			}
			g.focus(f.ID.index)
		}
		g.input.SetText(value)
		g.clearFieldError()
		g.previewInput()
		m.sync()
	})
}
func (m *desktopModel) commitField(f *desktopField) {
	g := m.editor
	if g.matchesField(f.ID) && g.active == f.ID.index {
		if !g.commit() {
			f.focus.Focus()
		}
	}
	m.sync()
}
func (m *desktopModel) selectTab(i int) {
	g := m.editor
	if i == g.tab {
		return
	}
	g.switchTab(i)
	m.sync()
}
func (m *desktopModel) source(raw bool) {
	m.editor.setRaw(raw)
	m.sync()
}
func (m *desktopModel) advanced() {
	m.editor.toggleAdvanced()
	m.sync()
}

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
		m.sync()
		return
	}
	g.componentIndex = i
	g.pkgRaw = false
	g.rebuild()
	m.sync()
}

func (g *editor) assetsChanged() {
	if g.desktop != nil {
		g.desktop.Assets.Update(func(v int) int { return v + 1 })
	}
}
