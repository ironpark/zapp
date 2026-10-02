package gui

import (
	"testing"

	"github.com/ironpark/ggui"
	"github.com/ironpark/zapp"
)

// fieldText is what the text field labelled label shows.
func fieldText(t *testing.T, p *ggui.Probe, label string) string {
	t.Helper()
	n, ok := p.Semantics().Find(ggui.RoleTextField, label)
	if !ok {
		t.Fatalf("no field %s", label)
	}
	return n.Value
}

// Typing into a field keeps the caret where it was, previews each
// keystroke in the DMG and commits once on Enter.
func TestTypingPreviewsAndCommits(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.rebuild()
	p := widgetProbe(t, g)
	tapTextField(t, p, "Title")
	p.Text("a")
	p.Text("b")
	p.Type(ggui.Mods{}, ggui.KeyBackspace)
	p.Text("c")
	p.Frame()
	if got := fieldText(t, p, "Title"); got != "ac" {
		t.Fatalf("field shows %q, want ac", got)
	}
	if g.s.Project.DMG.Title != "ac" {
		t.Fatalf("preview title = %q, want the draft", g.s.Project.DMG.Title)
	}
	if !g.dirty() || g.s.CanUndo() {
		t.Fatalf("draft: dirty %v, undo %v; want a dirty draft with nothing to undo", g.dirty(), g.s.CanUndo())
	}
	p.Type(ggui.Mods{}, ggui.KeyEnter)
	p.Frame()
	if g.active >= 0 || g.s.Project.DMG.Title != "ac" || !g.s.CanUndo() {
		t.Fatalf("after Enter: active %d, title %q, undo %v", g.active, g.s.Project.DMG.Title, g.s.CanUndo())
	}
	p.Tap("Undo")
	p.Frame()
	if g.s.Project.DMG.Title != "" || fieldText(t, p, "Title") != "" {
		t.Fatalf("undo left title %q, field %q", g.s.Project.DMG.Title, fieldText(t, p, "Title"))
	}
}

// An invalid draft keeps the last valid preview, stays in its field, and
// shows its error when committed; Escape puts the committed value back.
func TestInvalidDraftKeepsPreviewAndEscapeReverts(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.rebuild()
	p := widgetProbe(t, g)
	tapTextField(t, p, "Icon size")
	p.Text("9")
	p.Text("6")
	p.Frame()
	if g.s.Project.DMG.IconSize != 96 {
		t.Fatalf("preview icon size = %d, want 96", g.s.Project.DMG.IconSize)
	}
	p.Text("x")
	p.Frame()
	if g.s.Project.DMG.IconSize != 96 || fieldText(t, p, "Icon size") != "96x" {
		t.Fatalf("invalid draft: preview %d, field %q", g.s.Project.DMG.IconSize, fieldText(t, p, "Icon size"))
	}
	p.Type(ggui.Mods{}, ggui.KeyEnter)
	p.Frame()
	if g.active < 0 || shownError(g, "Icon size") == "" {
		t.Fatalf("invalid commit: active %d, error %q", g.active, shownError(g, "Icon size"))
	}
	p.Type(ggui.Mods{}, ggui.KeyEscape)
	p.Frame()
	if g.s.Project.DMG.IconSize != 0 || fieldText(t, p, "Icon size") != "" || g.dirty() {
		t.Fatalf("Escape left size %d, field %q, dirty %v", g.s.Project.DMG.IconSize, fieldText(t, p, "Icon size"), g.dirty())
	}
}

// A field's draft survives moving to another tab only when it commits:
// an invalid one keeps the user where they are.
func TestInvalidDraftBlocksTheTabSwitch(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.s.Project.DMG.Contents = map[string]zapp.Content{}
	g.rebuild()
	p := widgetProbe(t, g)
	tapTextField(t, p, "Window width")
	p.Text("z")
	p.Frame()
	p.Tap("Project")
	p.Frame()
	if g.tab != tabDMG || fieldText(t, p, "Window width") != "z" {
		t.Fatalf("tab %d, field %q; want the DMG tab still showing the draft", g.tab, fieldText(t, p, "Window width"))
	}
}

// shownError is the error shown under the field labelled label.
func shownError(g *editor, label string) string {
	for _, f := range g.desktop.Fields.Get() {
		if f.Spec.Label == label {
			return f.Error.Get()
		}
	}
	return ""
}
