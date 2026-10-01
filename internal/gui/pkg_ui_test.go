package gui

import (
	"testing"

	"github.com/ironpark/zapp"
)

func TestPackageSourceSwitchKeepsInvalidDraft(t *testing.T) {
	g := testEditor(t)
	g.tab = tabPKG
	g.s.Project.PKG.Components = []zapp.Component{{ID: "app", Root: "payload"}}
	g.rebuild()
	if !g.setRaw(true) || !g.pkgRaw {
		t.Fatal("did not switch to JSON")
	}
	g.focus(2)
	g.input.SetText("[")
	if g.setRaw(false) || !g.pkgRaw || g.input.Text() != "[" {
		t.Fatal("invalid JSON draft lost")
	}
}

func TestPackageEmptyComponentsLiveEditing(t *testing.T) {
	for _, empty := range []string{"", "[]", "null"} {
		t.Run(empty, func(t *testing.T) {
			g := testEditor(t)
			g.tab = tabPKG
			g.rebuild()
			g.switchPackageForm()
			g.pkgRaw = true
			g.rebuild()
			g.focus(2)
			g.input.SetText(empty)
			g.previewInput()
			if !g.commit() {
				t.Fatal(g.status)
			}
			g.focus(2)
			g.input.SetText(`[{"id":"new","root":"payload"}]`)
			g.previewInput()
			if !g.commit() {
				t.Fatal(g.status)
			}
			if len(g.s.Project.PKG.Components) != 1 || g.s.Project.PKG.Components[0].ID != "new" || g.s.Project.PKG.Identifier != "" {
				t.Fatal("JSON saved to wrong field")
			}
			g.history(false)
			if !g.s.Project.PKG.HasFullForm() {
				t.Fatal("undo lost empty component mode")
			}
		})
	}
}

func TestComponentEntryRejectsNestedPaths(t *testing.T) {
	value := "old.app"
	f := componentEntryField(&value)
	for _, invalid := range []string{"nested/App.app", "../App.app", `nested\App.app`, ".", "..", "bad\x00name"} {
		if f.set(invalid) == nil || value != "old.app" {
			t.Fatalf("accepted invalid entry %q", invalid)
		}
	}
	for _, valid := range []string{"", "Box00.app", "My App.app", "${app.name}.app"} {
		if err := f.set(valid); err != nil || value != valid {
			t.Fatalf("rejected %q: %v", valid, err)
		}
	}
}

func TestPackageDefaultsAreHints(t *testing.T) {
	g := testEditor(t)
	g.tab = tabPKG
	g.s.Project.PKG.Components = []zapp.Component{{ID: "app"}}
	g.rebuild()
	for _, f := range g.fields {
		if f.Label == "Install location" && (f.Placeholder != "/ (default)" || f.Value != "") {
			t.Fatal("wrong install default")
		}
		if f.Label == "Version" && (f.Placeholder != "1.0 (default)" || f.Value != "") {
			t.Fatal("wrong version default")
		}
	}
	if g.s.Project.PKG.Components[0].InstallLocation != "" || g.s.Project.PKG.Components[0].Version != "" {
		t.Fatal("hints changed project")
	}
}
