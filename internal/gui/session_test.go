package gui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ironpark/zapp"
)

func fixture(t *testing.T, name, contents string) *Session {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSavePreservesProjectAndRelativePaths(t *testing.T) {
	s := fixture(t, ".zapp.yaml", `version: 1
app: dist/MyApp.app
out: dist
dmg:
  background: artwork/background.png
  contents:
    dist/MyApp.app: {pos: [120, 200]}
    /Applications: {pos: [480, 200], link: true}
pkg:
  components:
    - {id: app, root: dist/MyApp.app, installLocation: /Applications}
  distribution:
    title: Installer
    choices:
      - {id: app, title: Application, packages: [app], selected: true, visible: true}
dep: {libs: [vendor/lib]}
sign: {identity: '${env:ZAPP_IDENTITY}'}
notarize: {profile: release, staple: true}
`)
	s.checkpoint()
	s.move("dist/MyApp.app", 222, 177)
	if !s.Dirty() {
		t.Fatal("move must mark document dirty")
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	p, err := zapp.Load(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	if p.App != "dist/MyApp.app" || p.DMG.Background != "artwork/background.png" || p.Sign.Identity != "${env:ZAPP_IDENTITY}" || p.Dep.Libs[0] != "vendor/lib" {
		t.Fatalf("paths/expressions changed: %+v", p)
	}
	if len(p.PKG.Components) != 1 || !p.PKG.Distribution.Choices[0].Selected || !p.Notarize.Staple {
		t.Fatal("unrelated settings lost")
	}
	if p.DMG.Contents[p.App].Pos[0] != 222 || p.DMG.Contents[p.App].Pos[1] != 177 {
		t.Fatal("coordinates not saved")
	}
	if s.Dirty() {
		t.Fatal("saved project is dirty")
	}
	info, _ := os.Stat(s.Path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("file permissions changed")
	}
	s.Undo()
	if s.Project.DMG.Contents[p.App].Pos[0] != 120 {
		t.Fatal("undo did not restore position")
	}
	s.Redo()
	if s.Project.DMG.Contents[p.App].Pos[0] != 222 {
		t.Fatal("redo did not restore position")
	}
}

func TestSaveDetectsExternalChanges(t *testing.T) {
	s := fixture(t, ".zapp.yaml", "version: 1\ndmg: {}\n")
	external := []byte("version: 1\ndmg: {title: external}\n")
	if err := os.WriteFile(s.Path, external, 0644); err != nil {
		t.Fatal(err)
	}
	s.Project.App = "App.app"
	if err := s.Save(); err == nil || !strings.Contains(err.Error(), "changed on disk") {
		t.Fatalf("expected conflict, got %v", err)
	}
	b, _ := os.ReadFile(s.Path)
	if string(b) != string(external) {
		t.Fatal("overwrote external edit")
	}
}

func TestNewJSONAndDisabledSections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("opening created a file")
	}
	s.Project.DMG = nil
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if !json.Valid(b) {
		t.Fatal(".json must remain JSON")
	}
	p, err := zapp.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Legacy() || p.DMG != nil {
		t.Fatal("all-disabled project reopened as legacy DMG")
	}
}

func TestNewFileConflict(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zapp.yaml")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("external"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(); err == nil {
		t.Fatal("new file overwrote external content")
	}
}

func TestDefaultLayoutMatchesBuildAndDrag(t *testing.T) {
	s := fixture(t, ".zapp.yaml", "version: 1\napp: Demo.app\ndmg: {}\n")
	if err := os.Mkdir(filepath.Join(filepath.Dir(s.Path), "Demo.app"), 0755); err != nil {
		t.Fatal(err)
	}
	pl, err := s.Project.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	items := buildLayout(s.Project.DMG, s.Project.App).Items
	for i, item := range items {
		if item.X != pl.DMG.Contents[i].X || item.Y != pl.DMG.Contents[i].Y {
			t.Fatal("preview defaults differ from build")
		}
	}
	s.checkpoint()
	s.move("Demo.app", -40, 9999)
	if s.Project.DMG.Contents["Demo.app"].Pos[0] != 0 || s.Project.DMG.Contents["Demo.app"].Pos[1] != 480 {
		t.Fatal("drag not clamped in content coordinates")
	}
	if !s.Project.DMG.Contents["/Applications"].Link {
		t.Fatal("lost Applications link")
	}
	s.Undo()
	if s.Project.DMG.Contents != nil {
		t.Fatal("undo should restore automatic layout")
	}
}

func TestSaveRejectsInvalidLayout(t *testing.T) {
	for _, change := range []func(*zapp.DMGConfig){
		func(c *zapp.DMGConfig) { c.IconSize = 8 },
		func(c *zapp.DMGConfig) { c.LabelSize = 2 },
		func(c *zapp.DMGConfig) { c.Window.Width = -1 },
		func(c *zapp.DMGConfig) { c.Format = "bad" },
		func(c *zapp.DMGConfig) { c.Contents = map[string]zapp.Content{"a": {}} },
	} {
		s := fixture(t, ".zapp.yaml", "version: 1\ndmg: {}\n")
		change(s.Project.DMG)
		if err := s.Save(); err == nil {
			t.Fatal("saved invalid layout")
		}
	}
}

func TestPreviewCoordinateRoundTrip(t *testing.T) {
	tr := previewTransform{x: 420, y: 240, scale: .375}
	x, y := tr.content(tr.x+180*tr.scale, tr.y+220*tr.scale)
	if x != 180 || y != 220 {
		t.Fatalf("content coordinates: %v,%v", x, y)
	}
}

func TestSaveThroughSymlinkKeepsConfigBase(t *testing.T) {
	s := fixture(t, "project.yaml", "version: 1\napp: Demo.app\ndmg: {}\n")
	dir := t.TempDir()
	link := filepath.Join(dir, ".zapp.yaml")
	if err := os.Symlink(s.Path, link); err != nil {
		t.Skip(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "Demo.app"), 0755); err != nil {
		t.Fatal(err)
	}
	linked, err := Open(link)
	if err != nil {
		t.Fatal(err)
	}
	pl, err := linked.Project.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if pl.App != filepath.Join(dir, "Demo.app") {
		t.Fatalf("relative base changed: %s", pl.App)
	}
	linked.Project.Out = "release"
	if err := linked.Save(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("save replaced the symlink")
	}
	p, err := zapp.Load(s.Path)
	if err != nil || p.Out != "release" {
		t.Fatalf("target not updated: %v", err)
	}
}

func TestPreviewSignatureIgnoresCoordinatesOnly(t *testing.T) {
	project := func(x, y int, name string) *zapp.Project {
		return &zapp.Project{App: "My.app", DMG: &zapp.DMGConfig{
			Background: "bg.png",
			Contents:   map[string]zapp.Content{"My.app": {Pos: &zapp.Position{x, y}, Name: name}},
		}}
	}
	sig := func(p *zapp.Project) string {
		return previewSignature(p, buildLayout(p.DMG, p.App).Items)
	}
	base := sig(project(10, 20, ""))
	if moved := sig(project(300, 400, "")); moved != base {
		t.Errorf("moving an icon changed the preview signature:\n got %q\nwant %q", moved, base)
	}
	if renamed := sig(project(10, 20, "Renamed")); renamed == base {
		t.Error("renaming an item left the preview signature unchanged")
	}
	changed := project(10, 20, "")
	changed.DMG.Background = "other.png"
	if sig(changed) == base {
		t.Error("changing the background left the preview signature unchanged")
	}
}
