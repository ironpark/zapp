package gui

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/ironpark/ggui"
	guiruntime "github.com/ironpark/ggui/runtime"
	"github.com/ironpark/zapp/internal/gui/comp"
)

// desktopModel separates reactive presentation state from project transactions.
// Values/errors update individual controls; structural changes rebuild only the
// workspace. No frame counter or absolute form geometry drives the widget tree.
type desktopModel struct {
	revealComponent                             int
	Assets                                      *ggui.StateValue[int]
	editor                                      *editor
	dialogs                                     guiruntime.FilePicker
	Workspace                                   *ggui.StateValue[workspaceState]
	Fields, Inspector                           *ggui.StateValue[[]*desktopField]
	Components                                  *ggui.StateValue[[]componentRow]
	Items                                       *ggui.StateValue[[]layoutItem]
	Status, SaveState                           *ggui.StateValue[string]
	Failed, CanUndo, CanRedo, Busy, Help, Close *ggui.StateValue[bool]
	Tab                                         *ggui.StateValue[int]
	Modal                                       *ggui.StateValue[modalState]
	Dark                                        *ggui.StateValue[bool]
	Split, InspectorSplit                       *ggui.StateValue[float64]
	DropHover                                   *ggui.StateValue[bool]
	cache                                       map[fieldIdentity]*desktopField
	scroll                                      map[string]*ggui.StateValue[float64]
	focus                                       *fieldIdentity
}
type workspaceState struct {
	Tab, Component, SignMethod, NotaryMethod            int
	Enabled, Full, Raw, Advanced, Actual, DefaultLayout bool
	Selected                                            string
	EnabledMask                                         uint8
	IssueTab                                            int
}
type componentRow struct {
	Index int
	Label string
}
type modalState struct {
	Title, Message                             string
	Build, Finished, Cancelling, Issue, Picker bool
}
type fieldIdentity struct {
	tab, component, index int
	item, label, syntax   string
}
type desktopField struct {
	ID           fieldIdentity
	Spec         comp.InputSpec
	Value, Error *ggui.StateValue[string]
}

func (g *editor) fieldIdentity(i int) fieldIdentity {
	f := g.fields[i]
	return fieldIdentity{g.tab, g.componentIndex, i, g.selected, f.Label, f.Syntax}
}
func (g *editor) matchesField(id fieldIdentity) bool {
	return id.index >= 0 && id.index < len(g.fields) && g.fieldIdentity(id.index) == id
}
func newDesktopModel(g *editor) *desktopModel {
	m := &desktopModel{
		editor:          g,
		revealComponent: -1,
		Assets:          ggui.State(0),
		Workspace:       ggui.State(workspaceState{}),
		Fields:          ggui.State([]*desktopField{}).WithEqual(slices.Equal),
		Inspector:       ggui.State([]*desktopField{}).WithEqual(slices.Equal),
		Components:      ggui.State([]componentRow{}).WithEqual(slices.Equal),
		Items:           ggui.State([]layoutItem{}).WithEqual(slices.Equal),
		Status:          ggui.State(""),
		SaveState:       ggui.State(""),
		Failed:          ggui.State(false),
		CanUndo:         ggui.State(false),
		CanRedo:         ggui.State(false),
		Busy:            ggui.State(false),
		Help:            ggui.State(g.helpOpen),
		Close:           ggui.State(false),
		Tab:             ggui.State(g.tab),
		Modal:           ggui.State(modalState{}),
		Dark:            ggui.State(true),
		Split:           ggui.State(0.3),
		InspectorSplit:  ggui.State(0.7),
		DropHover:       ggui.State(false),
		cache:           map[fieldIdentity]*desktopField{},
		scroll:          map[string]*ggui.StateValue[float64]{},
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
	v := workspaceState{Tab: g.tab, Component: g.componentIndex, Enabled: g.enabled(), Selected: g.selected, Actual: g.previewActual, IssueTab: -1}
	for i, s := range sections {
		if s.Enabled(g.s.Project) {
			v.EnabledMask |= 1 << i
		}
	}
	if g.issue != nil {
		v.IssueTab = g.issue.tab
	}
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
	m.Workspace.Set(v)
	m.Tab.Set(g.tab)
	m.Help.Set(g.helpOpen)
	m.Close.Set(g.confirmClose)
	m.Status.Set(g.status)
	m.Failed.Set(g.failed)
	m.CanUndo.Set(g.s.CanUndo())
	m.CanRedo.Set(g.s.CanRedo())
	m.Busy.Set(g.build != nil || g.picking != nil)
	state := "Saved"
	if !g.s.exists {
		state = "New project"
	}
	if g.dirty() {
		state = "Unsaved changes"
	}
	m.SaveState.Set(state)
	fields, inspector := []*desktopField{}, []*desktopField{}
	next := map[fieldIdentity]*desktopField{}
	for i, f := range g.fields {
		id := g.fieldIdentity(i)
		spec := f.InputSpec
		spec.Value = ""
		spec.Error = ""
		current := m.cache[id]
		if current == nil || !reflect.DeepEqual(current.Spec, spec) {
			current = &desktopField{ID: id, Spec: spec, Value: ggui.State(f.Value), Error: ggui.State(f.Error)}
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
	if g.picking != nil {
		modal = modalState{Title: "Choose a path", Message: "Select a path in the system dialog.", Picker: true}
	} else if g.build != nil {
		d := g.buildDialog()
		modal = modalState{Title: d.Title, Message: d.Message, Build: true, Finished: g.build.finished, Cancelling: g.build.cancelling, Issue: g.issue != nil}
	}
	m.Modal.Set(modal)
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
			id := f.ID
			m.focus = &id
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
	g := m.editor
	if !g.commit() {
		m.sync()
		return
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
	m.sync()
}
func (m *desktopModel) advanced() {
	g := m.editor
	if !g.commit() {
		m.sync()
		return
	}
	if g.tab == tabDMG {
		g.dmgAdvanced = !g.dmgAdvanced
	} else {
		g.pkgAdvanced = !g.pkgAdvanced
	}
	g.rebuild()
	m.sync()
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
