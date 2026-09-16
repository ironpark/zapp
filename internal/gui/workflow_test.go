package gui

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
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
	if len(g.fields) != 2 || g.s.Project.Sign.Identity != "" {
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
	results := make(chan pickResult, 1)
	results <- pickResult{index: 0, path: filepath.Join(g.s.Dir, "vendor")}
	g.picking = results
	g.pollPicker()
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
	for _, b := range g.workflowButtons() {
		if b.Label == "Remove path" {
			b.OnClick()
			break
		}
	}
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

func TestWorkflowLayoutAtMinimumSize(t *testing.T) {
	for _, tab := range []int{tabProject, tabPKG, tabDep, tabSign, tabNotarize} {
		for _, help := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/%v", tab, help), func(t *testing.T) {
				g := testEditor(t)
				g.w = 1080
				g.h = 720
				g.tab = tab
				g.helpOpen = help
				g.s.Project.Dep = &zapp.DepConfig{}
				g.s.Project.Sign = &zapp.SignConfig{}
				g.s.Project.Notarize = &zapp.NotarizeConfig{}
				if tab == tabPKG {
					g.s.Project.PKG.Components = []zapp.Component{{ID: "app"}}
				}
				g.rebuild()
				panel := g.settingsPanel().Bounds
				if panel.Dx() < 400 || !panel.In(image.Rect(0, 0, g.w, g.h)) {
					t.Fatalf("invalid panel: %v", panel)
				}
				for _, s := range g.workflowSegments() {
					if !s.Bounds.In(image.Rect(0, 0, g.w, g.h)) {
						t.Fatal("segment out of bounds")
					}
				}
				if g.componentListVisible() && panel.Overlaps(g.componentPanel().Bounds) {
					t.Fatal("list overlaps form")
				}
			})
		}
	}
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
	found := false
	for _, b := range g.buildDialog().Actions {
		if b.Label == "Go to issue" {
			found = true
			b.OnClick()
			break
		}
	}
	if !found || g.build != nil || g.tab != tabDep {
		t.Fatal("build dialog did not return to settings")
	}
}

func TestComponentScrollDoesNotChangeSelection(t *testing.T) {
	g := testEditor(t)
	g.tab = tabPKG
	g.s.Project.PKG.Components = make([]zapp.Component, 30)
	for i := range g.s.Project.PKG.Components {
		g.s.Project.PKG.Components[i].ID = fmt.Sprintf("app-%d", i)
	}
	g.rebuild()
	if !g.scrollComponents(g.componentPanel().Bounds.Min.Add(image.Pt(20, 60)), 10) || g.componentScroll != 10 || g.componentIndex != 0 {
		t.Fatal("scroll changed selection instead of viewport")
	}
	g.componentIndex = 29
	g.revealComponent()
	if g.componentScroll <= 10 {
		t.Fatal("newly selected component is not revealed")
	}
}
