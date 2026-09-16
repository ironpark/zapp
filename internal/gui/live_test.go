package gui

import "testing"

func TestLiveInputTransaction(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.rebuild()
	g.focus(2)
	for _, value := range []string{"8", "80", "800"} {
		g.input.SetText(value)
		g.previewInput()
	}
	if g.s.layout().W != 800 || g.active != 2 || g.input.Text() != "800" {
		t.Fatal("preview or focus not updated")
	}
	if g.s.CanUndo() {
		t.Fatal("typing created history")
	}
	g.input.SetText("-")
	g.previewInput()
	if g.s.layout().W != 800 {
		t.Fatal("invalid draft changed preview")
	}
	g.input.SetText("900")
	g.previewInput()
	if !g.commit() || g.s.layout().W != 900 {
		t.Fatal("commit failed")
	}
	g.history(false)
	if g.s.Project.DMG.Window.Width != 0 || g.s.CanUndo() {
		t.Fatal("typing should undo as one edit")
	}
	g.history(true)
	if g.s.layout().W != 900 {
		t.Fatal("redo failed")
	}
	g.focus(2)
	g.input.SetText("700")
	g.previewInput()
	g.rebuild()
	if g.s.layout().W != 900 || g.liveBase != nil {
		t.Fatal("cancel did not restore project")
	}
}

func TestLiveYAMLAndAdvanced(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.rebuild()
	for _, button := range g.controls() {
		if button.Label != "Advanced settings" {
			continue
		}
		if !button.Bounds.In(g.settingsPanel().Bounds) || button.Bounds.Overlaps(g.form.Bounds) {
			t.Fatal("advanced outside panel or overlaps inputs")
		}
		button.OnClick()
		if !g.dmgAdvanced || g.form.Offset() == 0 {
			t.Fatal("advanced did not expand and reveal fields")
		}
	}
	g.switchLayoutView(1)
	g.focus(0)
	g.input.SetText("title: Live\nwindow: {width: 800, height: 480}")
	g.previewInput()
	if g.s.layout().W != 800 || g.s.Project.DMG.Title != "Live" {
		t.Fatal("YAML preview failed")
	}
	g.input.SetText("window: [")
	g.previewInput()
	if g.s.layout().W != 800 {
		t.Fatal("invalid YAML changed preview")
	}
	g.rebuild()
	if g.s.Project.DMG.Title != "" {
		t.Fatal("YAML cancellation failed")
	}
}
