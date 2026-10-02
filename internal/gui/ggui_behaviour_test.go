package gui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ironpark/ggui"
	"github.com/ironpark/zapp"
)

// Save is the Save button's shortcut: it saves while the button could be
// pressed and does nothing while a dialog is open over it.
func TestSaveShortcutFollowsTheButton(t *testing.T) {
	g := testEditor(t)
	g.tab = tabProject
	g.rebuild()
	p := widgetProbe(t, g)
	setTextField(t, p, "Output directory", "elsewhere")
	g.desktop.commitField(g.desktop.Fields.Get()[fieldIndex(t, g, "Output directory")])
	if !g.s.Dirty() {
		t.Fatal("the edit left the project clean")
	}
	g.confirmClose = true
	g.desktop.sync()
	p.Frame()
	p.Key("cmd+s")
	if !g.s.Dirty() {
		t.Fatal("cmd+s saved behind the close dialog")
	}
	g.confirmClose = false
	g.desktop.sync()
	p.Frame()
	p.Key("cmd+s")
	if g.s.Dirty() || g.s.Project.Out != "elsewhere" {
		t.Fatalf("cmd+s did not save: dirty %v, out %q", g.s.Dirty(), g.s.Project.Out)
	}
}

// revealField focuses a field from outside paint, as Go to issue does.
func TestRevealFieldFocusesTheField(t *testing.T) {
	g := testEditor(t)
	g.tab = tabProject
	g.rebuild()
	p := widgetProbe(t, g)
	i := fieldIndex(t, g, "Output directory")
	g.revealField(i)
	p.Frame()
	p.Frame()
	if !g.desktop.fieldFocus(g.fieldIdentity(i)).Focused() {
		t.Fatal("revealField did not focus the field")
	}
}

// The build log follows new lines until the user scrolls up, stays put while
// they read, and follows again from the end.
func TestBuildLogFollowsTheEnd(t *testing.T) {
	g := testEditor(t)
	g.tab = tabProject
	g.rebuild()
	p := widgetProbe(t, g)
	g.build = &buildJob{}
	lines := 0
	grow := func(n int) float64 {
		t.Helper()
		for range n {
			g.build.log = append(g.build.log, fmt.Sprintf("build line %d", lines))
			lines++
		}
		g.build.logDirty = true
		g.desktop.sync()
		p.Frame()
		p.Frame()
		return g.desktop.offset("build-log").Get()
	}
	first := grow(40)
	if first <= 0 {
		t.Fatalf("offset %v with 40 lines, want the log scrolled to its end", first)
	}
	end := grow(40)
	if end <= first {
		t.Fatalf("offset %v after 40 more lines, want past %v", end, first)
	}
	var log ggui.Rect
	for _, n := range p.Semantics().All() {
		if strings.HasPrefix(n.Name, "build line 0") {
			log = n.ID.Rect
		}
	}
	if log == (ggui.Rect{}) {
		t.Fatal("no build log on screen")
	}
	// The text is laid out from the scroll's top, moved up by the offset.
	at := ggui.Pt(log.Origin.X+20, log.Origin.Y+end+20)
	p.Scroll(at, ggui.Pt(0, 3))
	back := g.desktop.offset("build-log").Get()
	if back >= end {
		t.Fatalf("offset %v after scrolling up from %v", back, end)
	}
	if got := grow(40); got != back {
		t.Fatalf("offset %v after new lines while scrolled up, want %v", got, back)
	}
	p.Scroll(at, ggui.Pt(0, -1000))
	bottom := g.desktop.offset("build-log").Get()
	if got := grow(40); got <= bottom {
		t.Fatalf("offset %v after new lines back at the end, want past %v", got, bottom)
	}
}

func fieldIndex(t *testing.T, g *editor, label string) int {
	t.Helper()
	for i, f := range g.fields {
		if f.Label == label {
			return i
		}
	}
	t.Fatalf("no field %q", label)
	return -1
}

// Selecting an item scrolls the contents list to its row, which is painted
// out of view until then.
func TestRevealItemScrollsTheContentsList(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.s.Project.DMG.Contents = map[string]zapp.Content{}
	for i := range 40 {
		g.s.Project.DMG.Contents[fmt.Sprintf("item-%02d.txt", i)] = zapp.Content{Pos: &zapp.Position{20 + i, 20}}
	}
	g.rebuild()
	p := widgetProbe(t, g)
	last := g.s.layout().Items[len(g.s.layout().Items)-1].Path
	g.selected = last
	g.invalidate()
	for range 4 {
		p.Frame()
	}
	if ggui.Untrack(g.desktop.offset("items").Get) <= 0 {
		t.Fatal("the contents list did not scroll to the selected item")
	}
}

// The contents list is a list box: a click selects the item, which it
// then reports as the selected option.
func TestContentsListSelectsTheClickedItem(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.s.Project.DMG.Contents = map[string]zapp.Content{
		"a.txt": {Pos: &zapp.Position{20, 20}},
		"b.txt": {Pos: &zapp.Position{120, 20}},
	}
	g.rebuild()
	p := widgetProbe(t, g)
	p.Tap("Item b.txt")
	p.Frame()
	if g.selected != "b.txt" {
		t.Fatalf("selected %q after clicking b.txt", g.selected)
	}
	n, ok := p.Semantics().Find(ggui.RoleOption, "Item b.txt")
	if !ok || !n.Selected {
		t.Fatalf("b.txt's option = %+v, %v; want it selected", n, ok)
	}
}
