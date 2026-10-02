package gui

import (
	"reflect"

	"github.com/ironpark/ggui"
)

// fieldIdentity tells a field apart from every other the editor builds, so
// its state outlives the rebuilds of the page around it.
type fieldIdentity struct {
	tab, component, index int
	item, label, syntax   string
}

// fieldState is what a field keeps while the page is rebuilt around it:
// the text its input shows, the error shown under it and the FocusRef that
// focuses it. The text is the committed value, or the draft while the field
// is being edited; the input is bound to it, so there is no other copy.
type fieldState struct {
	ID    fieldIdentity
	Spec  InputSpec // as built
	Text  *ggui.StateValue[string]
	Error *ggui.StateValue[string]
	focus *ggui.FocusRef
}

// shownError is the error shown under the field, if it is on the page.
func (f field) shownError() string {
	if f.state == nil {
		return ""
	}
	return f.state.Error.Get()
}

// keptDraft is a draft put back into its field when the page is rebuilt
// under it; see keepingDraft.
type keptDraft struct {
	id   fieldIdentity
	spec InputSpec
	text string
}

func (g *editor) fieldIdentity(i int) fieldIdentity {
	f := g.fields[i]
	return fieldIdentity{g.tab, g.componentIndex, i, g.selected, f.Label, f.Syntax}
}

func (g *editor) matchesField(id fieldIdentity) bool {
	return id.index >= 0 && id.index < len(g.fields) && g.fieldIdentity(id.index) == id
}

// fieldFocus is the FocusRef of the field with identity id, made before the
// field is built if need be, as revealField does after a tab switch.
func (g *editor) fieldFocus(id fieldIdentity) *ggui.FocusRef {
	if g.focusRefs == nil {
		g.focusRefs = map[fieldIdentity]*ggui.FocusRef{}
	}
	if g.focusRefs[id] == nil {
		g.focusRefs[id] = &ggui.FocusRef{}
	}
	return g.focusRefs[id]
}

// attachStates gives each field of the page its state: the one it had, by
// identity, while its presentation is unchanged, else a new one. Every field
// shows its committed value and no error, except a kept draft, which stays.
func (g *editor) attachStates() {
	next := make(map[fieldIdentity]*fieldState, len(g.fields))
	for i := range g.fields {
		id := g.fieldIdentity(i)
		spec := g.fields[i].InputSpec
		st := g.states[id]
		if st == nil || !sameLook(st.Spec, spec) {
			st = &fieldState{ID: id, Text: ggui.State(spec.Value), Error: ggui.State(""), focus: g.fieldFocus(id)}
		}
		st.Spec = spec
		text := spec.Value
		if g.kept != nil && g.kept.id == id {
			text = g.kept.text
			g.active = i
			g.input = Input{Spec: g.kept.spec, text: st.Text}
		}
		st.Text.Set(text)
		st.Error.Set("")
		g.fields[i].state = st
		next[id] = st
	}
	g.states = next
	for id := range g.focusRefs {
		if next[id] == nil {
			delete(g.focusRefs, id)
		}
	}
}

// sameLook reports whether two specs present a field alike, whatever its
// value.
func sameLook(a, b InputSpec) bool {
	a.Value, b.Value = "", ""
	return reflect.DeepEqual(a, b)
}

// keepingDraft runs rebuild, which rebuilds the page, and puts the field
// being edited and its draft back into it, so a live preview or a commit
// that failed leaves the user where they were.
func (g *editor) keepingDraft(rebuild func()) {
	if g.active < 0 {
		rebuild()
		return
	}
	g.kept = &keptDraft{g.fieldIdentity(g.active), g.input.Spec, g.input.Text()}
	defer func() { g.kept = nil }()
	rebuild()
}
