package gui

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ironpark/zapp"
)

func TestDependencyLinesCommitAndUndo(t *testing.T) {
	g := testEditor(t)
	g.s.Project.Dep = &zapp.DepConfig{}
	g.tab = tabDep
	g.rebuild()
	g.focus(0)
	g.input.SetText(" vendor/My Libraries \r\n\n/opt/lib\n")
	if !g.commit() || !reflect.DeepEqual(g.s.Project.Dep.Libs, []string{"vendor/My Libraries", "/opt/lib"}) {
		t.Fatalf("paths not parsed: %+v", g.s.Project.Dep)
	}
	g.history(false)
	if len(g.s.Project.Dep.Libs) != 0 {
		t.Fatal("undo did not restore empty paths")
	}
	g.history(true)
	if len(g.s.Project.Dep.Libs) != 2 {
		t.Fatal("redo lost paths")
	}
}

func TestStapleToggleAndUndo(t *testing.T) {
	g := testEditor(t)
	g.s.Project.Notarize = &zapp.NotarizeConfig{}
	g.tab = tabNotarize
	g.rebuild()
	g.focus(4)
	g.openChoice()
	if !g.s.Project.Notarize.Staple || g.choiceOpen {
		t.Fatal("toggle opened a menu or did not apply")
	}
	g.history(false)
	if g.s.Project.Notarize.Staple {
		t.Fatal("undo did not restore staple")
	}
}

func TestDefaultPackageFormRoundTrip(t *testing.T) {
	g := testEditor(t)
	g.tab = tabPKG
	g.rebuild()
	g.switchPackageForm()
	if !g.s.Project.PKG.HasFullForm() {
		t.Fatal("did not switch to components")
	}
	g.switchPackageForm()
	if g.s.Project.PKG.HasFullForm() {
		t.Fatal("default component prevented switching back")
	}
	g.switchPackageForm()
	g.s.Project.PKG.Components[0].Root = "custom-root"
	g.switchPackageForm()
	if !g.failed || g.s.Project.PKG.Components[0].Root != "custom-root" {
		t.Fatal("custom component was discarded")
	}
}

func TestOutputDirectoryValidation(t *testing.T) {
	g := testEditor(t)
	g.s.Project.Out = filepath.Join(g.s.Dir, "new-output")
	if !g.validatePaths() {
		t.Fatalf("new directory rejected: %s", g.status)
	}
	if err := os.WriteFile(g.s.Project.Out, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	if g.validatePaths() || g.tab != tabProject || g.fields[g.active].Label != "Output directory" {
		t.Fatal("file accepted as output directory")
	}
}

func TestOptionalJSONCanBeCleared(t *testing.T) {
	value := map[string]string{"default": "license.txt"}
	f := jsonField("Licenses", &value, "")
	if err := f.set("  \n"); err != nil || value != nil {
		t.Fatalf("clear failed: %v, %#v", err, value)
	}
	if empty := jsonField("Licenses", &value, ""); empty.Value != "" {
		t.Fatal("nil shown as raw null")
	}
	if err := f.set(`{"default":"license.txt","extra":`); err == nil || value != nil {
		t.Fatal("invalid draft changed value")
	}
}
