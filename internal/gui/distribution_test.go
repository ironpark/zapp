package gui

import (
	"slices"
	"strings"
	"testing"
)

func (g *editor) focusLabel(t *testing.T, label string) {
	t.Helper()
	i := slices.IndexFunc(g.fields, func(f field) bool { return f.Label == label })
	if i < 0 {
		t.Fatalf("no %q field", label)
	}
	g.focus(i)
}

func (g *editor) enter(t *testing.T, label, value string) bool {
	t.Helper()
	g.focusLabel(t, label)
	g.input.SetText(value)
	return g.commit()
}

// ZIP is switched on and off, keeping what it held, and its output field
// shows only while it is on.
func TestDistributionZipToggle(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDistribution
	g.rebuild()
	if slices.ContainsFunc(g.fields, func(f field) bool { return f.Label == "ZIP output" }) {
		t.Fatal("ZIP output shown while ZIP is off")
	}
	if !g.enter(t, "Archive as ZIP", "true") || g.s.Project.Zip == nil {
		t.Fatal("ZIP not switched on")
	}
	if !g.enter(t, "ZIP output", "dist/App.zip") {
		t.Fatal(g.status)
	}
	if !g.enter(t, "Archive as ZIP", "false") || g.s.Project.Zip != nil {
		t.Fatal("ZIP not switched off")
	}
	if !g.enter(t, "Archive as ZIP", "true") || g.s.Project.Zip == nil || g.s.Project.Zip.Out != "dist/App.zip" {
		t.Fatalf("ZIP came back as %+v", g.s.Project.Zip)
	}
	g.history(false)
	if g.s.Project.Zip != nil {
		t.Fatal("undo did not switch ZIP off")
	}
}

// Uploads and the appcast are checked as a project file's would be.
func TestDistributionYAML(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDistribution
	g.rebuild()
	uploads := "- url: https://example.com/${file.name}\n  headers:\n    Authorization: Bearer ${env:TOKEN}\n  artifacts: [zip]\n- github: {tag: v1}"
	if !g.enter(t, "Uploads", uploads) {
		t.Fatal(g.status)
	}
	if u := g.s.Project.Upload; len(u) != 2 || u[0].Headers["Authorization"] != "Bearer ${env:TOKEN}" || u[1].GitHub == nil || u[1].GitHub.Tag != "v1" {
		t.Fatalf("uploads = %+v", u)
	}
	for _, bad := range []string{
		"- url: https://example.com/\n  headers: {Authorization: Bearer literal}",
		"- url: https://example.com/\n  artifacts: [exe]",
		"- method: PUT",
		"url: https://example.com/",
	} {
		if g.enter(t, "Uploads", bad) {
			t.Errorf("accepted:\n%s", bad)
		}
		g.rebuild()
	}
	if len(g.s.Project.Upload) != 2 {
		t.Fatal("a rejected edit changed the uploads")
	}
	if !g.enter(t, "Sparkle appcast", "url: https://example.com/${file.name}\nartifact: dmg") || g.s.Project.Appcast == nil || g.s.Project.Appcast.Artifact != "dmg" {
		t.Fatalf("appcast = %+v, %s", g.s.Project.Appcast, g.status)
	}
	if g.enter(t, "Sparkle appcast", "artifact: dmg") {
		t.Error("accepted an appcast without url")
	}
	g.rebuild()
	if !g.enter(t, "Uploads", "  ") || g.s.Project.Upload != nil {
		t.Fatal("blank did not clear the uploads")
	}
	// The field shows the uploads as they would be saved.
	g.s.Project.Upload = nil
	g.enter(t, "Uploads", uploads)
	g.rebuild()
	i := slices.IndexFunc(g.fields, func(f field) bool { return f.Label == "Uploads" })
	if !strings.Contains(g.fields[i].Value, "tag: v1") || !strings.Contains(g.fields[i].Value, "${file.name}") {
		t.Fatalf("shown as:\n%s", g.fields[i].Value)
	}
}

func TestDistributionErrorsFindTheirField(t *testing.T) {
	g := testEditor(t)
	for message, want := range map[string]string{
		"upload[0]: github needs repo (owner/name), or GITHUB_REPOSITORY set": "Uploads",
		"appcast requires url, where the artifact is downloaded from":         "Sparkle appcast",
	} {
		loc, ok := g.locateIssue(errorString(message))
		if !ok || loc.tab != tabDistribution || loc.label != want {
			t.Errorf("%s -> %+v", message, loc)
		}
	}
}

type errorString string

func (e errorString) Error() string { return string(e) }
