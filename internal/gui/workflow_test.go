package gui

import (
	"encoding/json"
	"fmt"
	"github.com/ironpark/ggui"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ironpark/zapp"
)

func TestAuthenticationSwitchRestoresValuesAndUndo(t *testing.T) {
	g := testEditor(t)
	g.s.Project.Sign = &zapp.SignConfig{Identity: "Developer ID"}
	g.tab = tabSign
	g.rebuild()
	g.selectSignMethod(1)
	// Certificate and password file, then the entitlements every method has.
	if len(g.fields) != 3 || g.s.Project.Sign.Identity != "" {
		t.Fatal("keychain credentials stayed active")
	}
	g.focus(0)
	g.input.SetText("certificate.p12")
	if !g.commit() {
		t.Fatal(g.status)
	}
	g.selectSignMethod(0)
	if g.s.Project.Sign.Identity != "Developer ID" || g.s.Project.Sign.P12File != "" {
		t.Fatal("method restore failed")
	}
	g.selectSignMethod(1)
	if g.s.Project.Sign.P12File != "certificate.p12" {
		t.Fatal("certificate lost across methods")
	}
	g.history(false)
	if g.signMethod() != 0 || g.fields[0].Label != "Signing identity" {
		t.Fatal("undo did not restore method")
	}
}

func TestNotaryPasswordIsSessionOnly(t *testing.T) {
	g := testEditor(t)
	g.s.Project.Notarize = &zapp.NotarizeConfig{Profile: "profile", Staple: true}
	g.tab = tabNotarize
	g.rebuild()
	g.selectNotaryMethod(1)
	g.focus(0)
	g.input.SetText("test@example.com")
	g.commit()
	g.focus(2)
	if !g.input.Spec.Secret {
		t.Fatal("password is not masked")
	}
	g.input.SetText("test-password")
	g.commit()
	g.selectNotaryMethod(0)
	g.selectNotaryMethod(1)
	if g.s.Project.Notarize.Password != "test-password" || !g.s.Project.Notarize.Staple {
		t.Fatal("method switch lost runtime values")
	}
	data, err := json.Marshal(g.s.Project)
	if err != nil || strings.Contains(string(data), "test-password") {
		t.Fatal("password serialized")
	}
}

func TestDependencyListPickerRemoveAndRaw(t *testing.T) {
	g := testEditor(t)
	g.s.Project.Dep = &zapp.DepConfig{}
	g.tab = tabDep
	g.rebuild()
	g.picked(0, filepath.Join(g.s.Dir, "vendor"), nil)
	if len(g.s.Project.Dep.Libs) != 1 || !g.fields[1].Browse {
		t.Fatal("picker did not append directory")
	}
	g.depRaw = true
	g.rebuild()
	if g.fields[0].Value != "vendor" {
		t.Fatalf("raw did not retain path: %q", g.fields[0].Value)
	}
	g.depRaw = false
	g.rebuild()
	removeLibrary(g, 0)
	if len(g.s.Project.Dep.Libs) != 0 {
		t.Fatal("row removal failed")
	}
	g.history(false)
	if len(g.s.Project.Dep.Libs) != 1 {
		t.Fatal("undo lost removed path")
	}
}

func TestComponentEditingAndReferenceProtection(t *testing.T) {
	g := testEditor(t)
	g.tab = tabPKG
	g.rebuild()
	g.switchPackageForm()
	g.addComponent()
	if len(g.s.Project.PKG.Components) != 2 || g.componentIndex != 1 {
		t.Fatal("add failed")
	}
	for i, f := range g.fields {
		if f.Label == "Root directory" {
			g.focus(i)
			g.input.SetText("payload")
			g.commit()
			break
		}
	}
	if g.s.Project.PKG.Components[1].Root != "payload" {
		t.Fatal("wrong component edited")
	}
	id := g.s.Project.PKG.Components[1].ID
	g.s.Project.PKG.Distribution = &zapp.Distribution{Choices: []zapp.Choice{{Packages: []string{id}}}}
	g.removeComponent()
	if len(g.s.Project.PKG.Components) != 2 || !g.failed {
		t.Fatal("referenced component removed")
	}
	g.s.Project.PKG.Distribution = nil
	g.removeComponent()
	g.history(false)
	if len(g.s.Project.PKG.Components) != 2 || g.s.Project.PKG.Components[1].Root != "payload" {
		t.Fatal("undo lost component")
	}
}

func TestHiddenAdvancedPathValidationAndIssueReturn(t *testing.T) {
	g := testEditor(t)
	g.s.Project.PKG.Scripts = "missing-scripts"
	g.tab = tabProject
	g.rebuild()
	if g.validatePaths() || !g.pkgAdvanced || g.issue == nil || g.fields[g.active].Label != "Scripts directory" {
		t.Fatal("hidden path not revealed")
	}
	g.switchTab(tabProject)
	g.goToIssue()
	if g.tab != tabPKG || g.fields[g.active].Label != "Scripts directory" {
		t.Fatal("issue return failed")
	}
	if err := os.Mkdir(filepath.Join(g.s.Dir, "scripts"), 0700); err != nil {
		t.Fatal(err)
	}
	g.input.SetText("scripts")
	g.goToIssue() // Applying a fix must not dereference a cleared issue.
}

func TestDefaultComponentContainsAppBundle(t *testing.T) {
	g := testEditor(t)
	g.s.Project.App = "build/Demo.app"
	g.tab = tabPKG
	g.rebuild()
	g.switchPackageForm()
	c := g.s.Project.PKG.Components[0]
	if c.Root != "build" || c.Entry != "Demo.app" {
		t.Fatalf("app would be unpacked at install location: %+v", c)
	}
}

func TestRawComponentsRemainEditableWhenCleared(t *testing.T) {
	g := testEditor(t)
	g.tab = tabPKG
	g.rebuild()
	g.switchPackageForm()
	g.pkgRaw = true
	g.rebuild()
	g.focus(2)
	g.input.SetText("")
	if !g.commit() {
		t.Fatal(g.status)
	}
	if !g.s.Project.PKG.HasFullForm() || !g.pkgRaw || g.fields[2].Label != "Components" {
		t.Fatal("clear unexpectedly switched package mode")
	}
	g.focus(2)
	g.input.SetText(`[{"id":"app","root":"payload"}]`)
	if !g.commit() {
		t.Fatal(g.status)
	}
	if len(g.s.Project.PKG.Components) != 1 {
		t.Fatal("could not restore components")
	}
}

func TestBuildErrorOffersIssueNavigation(t *testing.T) {
	g := testEditor(t)
	g.s.Project.Dep = &zapp.DepConfig{}
	g.rebuild()
	job := &buildJob{cancel: func() {}, events: make(chan string), done: make(chan buildResult, 1)}
	job.done <- buildResult{err: fmt.Errorf("dep: no dependencies found")}
	g.build = job
	g.pollBuild()
	if g.issue == nil || g.issue.tab != tabDep {
		t.Fatal("build error did not identify step")
	}
	m := newDesktopModel(g)
	if v := m.Modal.Get(); !v.Finished || !v.Issue {
		t.Fatal("build dialog does not offer Go to issue")
	}
	g.dismissBuild()
	g.goToIssue()
	if g.build != nil || g.tab != tabDep {
		t.Fatal("build dialog did not return to settings")
	}
}

func TestRevealComponentScrollsToSelection(t *testing.T) {
	g := testEditor(t)
	g.tab = tabPKG
	g.s.Project.PKG.Components = make([]zapp.Component, 30)
	for i := range g.s.Project.PKG.Components {
		g.s.Project.PKG.Components[i].ID = fmt.Sprintf("app-%d", i)
	}
	g.rebuild()
	p := widgetProbe(t, g)
	g.componentIndex = 29
	g.revealComponent()
	for range 4 {
		p.Frame()
	}
	// FocusRef.Focused stays false after a focus that scrolled, until the
	// next input; the scroll is what shows the row was reached.
	if ggui.Untrack(g.desktop.offset("components").Get) <= 0 {
		t.Fatal("newly selected component is not revealed")
	}
}

// Refreshing looks the keychain up again and keeps the identities offered
// until the new list arrives.
func TestRefreshIdentities(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("only macOS has a keychain")
	}
	g := testEditor(t)
	g.ctx, g.tab, g.s.Project.Sign = t.Context(), tabSign, &zapp.SignConfig{}
	g.signing.identities, g.signing.listed = "Developer ID Application: A", true
	g.refreshIdentities()
	if g.signing.identities == "" || !g.signing.listed {
		t.Fatal("refresh dropped the identities offered")
	}
	if !g.signing.listing {
		t.Fatal("refresh did not ask for a new list")
	}
}
