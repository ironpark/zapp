package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ironpark/ggui"
	"github.com/ironpark/zapp"
)

// testApp writes a minimal app bundle next to the project and returns its
// path relative to the project file.
func testApp(t *testing.T, g *editor) string {
	t.Helper()
	app := filepath.Join(filepath.Dir(g.s.Path), "Demo.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o700); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0"?><plist version="1.0"><dict>` +
		`<key>CFBundleName</key><string>Demo</string>` +
		`<key>CFBundleShortVersionString</key><string>2.1</string>` +
		`<key>CFBundleIdentifier</key><string>com.example.demo</string>` +
		`<key>CFBundleExecutable</key><string>Demo</string>` +
		`</dict></plist>`
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte(plist), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "Contents", "MacOS", "Demo"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return "Demo.app"
}

// The project is checked as it changes, without moving the user, and each
// problem is filed under the tab that owns it.
func TestHealthTracksProjectWithoutNavigating(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.rebuild()
	if g.health.ready() || g.health.Issues[tabProject] == "" {
		t.Fatalf("a project without an app looks ready: %+v", g.health.Issues)
	}

	g.s.checkpoint()
	g.s.Project.App = testApp(t, g)
	g.rebuild()
	if g.tab != tabDMG {
		t.Fatal("checking moved the user")
	}
	if app := g.health.App; app.Name != "Demo" || app.Version != "2.1" || app.BundleID != "com.example.demo" {
		t.Fatalf("app summary = %+v", app)
	}
	if g.health.Issues[tabProject] != "" {
		t.Fatalf("app still reported missing: %s", g.health.Issues[tabProject])
	}
	if !strings.HasSuffix(g.health.Outputs[tabDMG], ".dmg") {
		t.Fatalf("DMG output = %q", g.health.Outputs[tabDMG])
	}

	g.s.checkpoint()
	g.s.Project.DMG.Background = "missing.png"
	g.rebuild()
	if !strings.Contains(g.health.Issues[tabDMG], "missing.png") || g.health.Count == 0 {
		t.Fatalf("missing background not filed under DMG: %+v", g.health.Issues)
	}
	if g.issue != nil || g.fields[0].Error != "" {
		t.Fatal("checking marked a field")
	}

	// Validate still moves to the problem the badge counts.
	g.tab = tabProject
	g.rebuild()
	g.validate()
	if g.tab != tabDMG || g.issue == nil {
		t.Fatal("validate did not open the problem")
	}
}

// A tab carries its step's problem, and the strip still switches tabs.
func TestStepTabsShowIssuesAndNavigate(t *testing.T) {
	g := testEditor(t)
	g.rebuild()
	p := widgetProbe(t, g)
	if _, ok := p.Find("Signing"); !ok {
		t.Fatal("tab strip lost its labels")
	}
	p.Tap("PKG")
	if g.tab != tabPKG {
		t.Fatalf("tab = %d", g.tab)
	}
	if _, ok := p.Find("Validate"); !ok {
		t.Fatal("health badge missing")
	}
}

// A dropped app bundle or certificate lands in the setting it belongs to.
func TestWindowDropRoutesFiles(t *testing.T) {
	g := testEditor(t)
	dir := filepath.Dir(g.s.Path)
	g.dropFiles([]string{filepath.Join(dir, "Other.app")})
	if g.s.Project.App != "Other.app" || g.tab != tabProject {
		t.Fatalf("app drop: app=%q tab=%d", g.s.Project.App, g.tab)
	}
	g.dropFiles([]string{filepath.Join(dir, "cert.p12")})
	if c := g.s.Project.Sign; c == nil || c.P12File != "cert.p12" || g.tab != tabSign || g.signMethod() != 1 {
		t.Fatalf("p12 drop: %+v tab=%d", c, g.tab)
	}
	g.dropFiles([]string{filepath.Join(dir, "bundle.pem")})
	if c := g.s.Project.Sign; c.PEMFile != "bundle.pem" || c.P12File != "" || g.signMethod() != 2 {
		t.Fatalf("pem drop: %+v", c)
	}
	g.history(false)
	if g.s.Project.Sign.P12File != "cert.p12" {
		t.Fatal("drop is not undoable")
	}
	g.dropFiles([]string{filepath.Join(dir, "notes.txt")})
	if !g.failed {
		t.Fatal("unknown file accepted")
	}
}

// A long build keeps its most recent lines.
func TestBuildLogIsBounded(t *testing.T) {
	j := &buildJob{}
	for i := range maxBuildLog + 5 {
		j.record(fmt.Sprint(i))
	}
	if len(j.log) != maxBuildLog || j.log[0] != "5" {
		t.Fatalf("log kept %d lines from %q", len(j.log), j.log[0])
	}
}

// Forms stay readable in a wide window.
func TestFormsHaveMaximumWidth(t *testing.T) {
	g := testEditor(t)
	g.w = 1600
	g.tab = tabProject
	g.rebuild()
	p := widgetProbe(t, g)
	f, ok := p.FindRole(ggui.RoleTextField, "Output directory")
	if !ok || f.Rect.Size.W > formMaxWidth {
		t.Fatalf("field is %v wide", f.Rect.Size.W)
	}
}

// A credential check describes the settings it ran against; once they
// change, its result is no longer shown.
func TestSigningCheckGoesStale(t *testing.T) {
	g := testEditor(t)
	g.s.Project.Sign = &zapp.SignConfig{Identity: "A"}
	checked := *g.s.Project.Sign
	g.signing.checked, g.signing.ok, g.signing.message = &checked, true, "Signs with A"
	if shown, _, ok, _ := g.signingCheck(); !shown || !ok {
		t.Fatal("current result hidden")
	}
	g.s.Project.Sign.Identity = "B"
	if shown, _, _, _ := g.signingCheck(); shown {
		t.Fatal("stale result shown")
	}
}
