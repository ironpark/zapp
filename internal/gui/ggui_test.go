package gui

import (
	"github.com/ironpark/ggui"
	guiruntime "github.com/ironpark/ggui/runtime"
	uitheme "github.com/ironpark/ggui/ui/theme"
	"github.com/ironpark/zapp"
	"os"
	"path/filepath"
	"testing"
)

func widgetProbe(t *testing.T, g *editor) *ggui.Probe {
	t.Helper()
	m := newDesktopModel(g)
	p := ggui.ProbeBuilder(func() ggui.Widget { return desktopView(m) }, ggui.Sz(g.w, g.h))
	m.dialogs = p.Dialogs()
	p.Setup(func() { uitheme.Set(editorTheme(true)) })
	t.Cleanup(p.Close)
	p.Frame()
	return p
}
func TestWidgetsEditSaveUndo(t *testing.T) {
	g := testEditor(t)
	g.tab = tabProject
	g.rebuild()
	p := widgetProbe(t, g)
	tapTextField(t, p, "Output directory")
	setTextField(t, p, "Output directory", "new output")

	p.Tap("Save")
	if g.s.Project.Out != "new output" || g.s.Dirty() {
		t.Fatalf("save did not commit widget: %q %s", g.s.Project.Out, g.status)
	}
	p.Tap("Undo")
	if g.s.Project.Out == "new output" {
		t.Fatal("project undo lost")
	}
}
func TestWidgetsRejectInvalidJSONBeforeSwitch(t *testing.T) {
	g := testEditor(t)
	g.tab = tabPKG
	g.s.Project.PKG.Components = []zapp.Component{{ID: "app", Root: "payload"}}
	g.pkgRaw = true
	g.rebuild()
	p := widgetProbe(t, g)
	tapTextField(t, p, "Components")
	setTextField(t, p, "Components", "[")
	p.Tap("Form")
	if !g.pkgRaw || g.active < 0 || g.input.Text() != "[" {
		t.Fatalf("invalid draft lost: raw=%v active=%d text=%s status=%s", g.pkgRaw, g.active, g.input.Text(), g.status)
	}
	tapTextField(t, p, "Components")
	setTextField(t, p, "Components", `[{"id":"replacement","root":"payload"}]`)
	p.Tap("Form")
	if g.pkgRaw || len(g.s.Project.PKG.Components) != 1 || g.s.Project.PKG.Components[0].ID != "replacement" {
		t.Fatalf("repair did not commit: %s", g.status)
	}
}
func TestWidgetsHelpAndModal(t *testing.T) {
	g := testEditor(t)
	g.tab = tabProject
	g.rebuild()
	p := widgetProbe(t, g)
	wide, _ := p.FindRole(ggui.RoleTextField, "Output directory")
	p.Tap("Help")
	narrow, _ := p.FindRole(ggui.RoleTextField, "Output directory")
	if !g.helpOpen || narrow.Rect.Size.W >= wide.Rect.Size.W {
		t.Fatal("help did not reserve space")
	}
	p.Tap("Help")
	restored, _ := p.FindRole(ggui.RoleTextField, "Output directory")
	if restored.Rect != wide.Rect {
		t.Fatal("settings did not reclaim space")
	}
	g.confirmClose = true
	g.invalidate()
	p.Frame()
	p.Tap("Discard changes")
	if !g.quit {
		t.Fatal("dialog action failed")
	}
}
func TestWidgetsStaleFieldBinding(t *testing.T) {
	g := testEditor(t)
	g.tab = tabPKG
	g.s.Project.PKG.Components = []zapp.Component{{ID: "first"}, {ID: "second"}}
	g.rebuild()
	m := newDesktopModel(g)
	binding := m.fieldBinding(m.cache[g.fieldIdentity(2)])
	g.componentIndex = 1
	g.rebuild()
	binding.Set("stale")
	if g.s.Project.PKG.Components[1].ID != "second" {
		t.Fatal("removed widget wrote into next component")
	}
}

func tapTextField(t *testing.T, p *ggui.Probe, label string) {
	t.Helper()
	f, ok := p.FindRole(ggui.RoleTextField, label)
	if !ok {
		t.Fatalf("no field %s", label)
	}
	p.Click(f.Center())
}

func setTextField(t *testing.T, p *ggui.Probe, label, value string) {
	t.Helper()
	for _, n := range p.Semantics().Nodes(ggui.RoleTextField) {
		if n.Role == ggui.RoleTextField && n.Name == label {
			p.Perform(n.ID, ggui.Action{Kind: ggui.ActionSetValue, Text: value})
			return
		}
	}
	t.Fatalf("no field %s", label)
}

func TestWidgetsKeyboardRevealAndNumber(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.dmgAdvanced = true
	g.rebuild()
	p := widgetProbe(t, g)
	tapTextField(t, p, "Icon size")
	p.Type(ggui.Mods{}, ggui.KeyArrowUp)
	p.Type(ggui.Mods{}, ggui.KeyEnter)
	if g.s.Project.DMG.IconSize != zapp.DefaultIconSize+1 {
		t.Fatalf("number step failed: %d", g.s.Project.DMG.IconSize)
	}
	n, ok := p.Semantics().Find(ggui.RoleTextField, "Output file")
	if !ok {
		t.Fatal("offscreen input missing")
	}
	p.Perform(n.ID, ggui.Action{Kind: ggui.ActionScrollIntoView})
	if g.desktop.offset("form:1:false:0").Get() == 0 {
		t.Fatal("keyboard reveal did not scroll")
	}
}
func TestWidgetsEverySectionAtMinimumSize(t *testing.T) {
	g := testEditor(t)
	g.s.Project.Dep = &zapp.DepConfig{}
	g.s.Project.Sign = &zapp.SignConfig{}
	g.s.Project.Notarize = &zapp.NotarizeConfig{}
	g.w, g.h = 1080, 720
	p := widgetProbe(t, g)
	for i := range sections {
		g.switchTab(i)
		g.invalidate()
		p.Frame()
		for _, n := range p.Semantics().Nodes(ggui.RoleTextField) {
			if !n.Offscreen && (n.Rect.Origin.X < 0 || n.Rect.Origin.X+n.Rect.Size.W > 1080) {
				t.Fatalf("section %d input exceeds window: %s", i, n.Name)
			}
		}
	}
}

func TestDesktopNativePickerAndPathDrop(t *testing.T) {
	g := testEditor(t)
	g.tab = tabProject
	g.rebuild()
	p := widgetProbe(t, g)
	dir := filepath.Join(filepath.Dir(g.s.Path), "output")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	picker := &guiruntime.StubFilePicker{Paths: []string{dir}}
	g.desktop.dialogs = picker
	p.Tap("Browse Output directory")
	if g.s.Project.Out != "output" || len(picker.Asked) != 1 {
		t.Fatalf("picker binding: %q %+v", g.s.Project.Out, picker.Asked)
	}
	before := g.s.Project.Out
	picker.Paths = nil
	p.Tap("Browse Output directory")
	if g.s.Project.Out != before {
		t.Fatal("cancel changed path")
	}
	other := filepath.Join(filepath.Dir(g.s.Path), "drop")
	if err := os.Mkdir(other, 0700); err != nil {
		t.Fatal(err)
	}
	f, _ := p.FindRole(ggui.RoleTextField, "Output directory")
	p.DropPaths(f.Center(), other)
	if g.s.Project.Out != "drop" {
		t.Fatalf("path drop not committed: %s", g.status)
	}
}
func TestDesktopCanvasDropDragAndUndo(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.s.Project.DMG.Contents = map[string]zapp.Content{}
	g.rebuild()
	p := widgetProbe(t, g)
	path := filepath.Join(filepath.Dir(g.s.Path), "payload.txt")
	if err := os.WriteFile(path, []byte("payload"), 0600); err != nil {
		t.Fatal(err)
	}
	canvas, ok := p.Find("DMG preview canvas")
	if !ok {
		t.Fatal("missing native canvas")
	}
	p.DropPaths(canvas.Center(), path)
	if len(g.s.Project.DMG.Contents) != 1 {
		t.Fatalf("drop failed: %s", g.status)
	}
	item := g.s.layout().Items[0]
	tr := g.transform()
	at := ggui.Pt(canvas.Rect.Origin.X+tr.x+float64(item.X)*tr.scale, canvas.Rect.Origin.Y+tr.y+float64(item.Y)*tr.scale)
	p.Press(at)
	p.Move(ggui.Pt(at.X+20, at.Y+10))
	p.Release(ggui.Pt(at.X+20, at.Y+10))
	moved := g.s.Project.DMG.Contents[item.Path]
	if moved.Pos == nil || moved.Pos[0] == item.X {
		t.Fatal("drag did not move item")
	}
	p.Tap("Undo")
	restored := g.s.Project.DMG.Contents[item.Path]
	if restored.Pos[0] != item.X || restored.Pos[1] != item.Y {
		t.Fatal("drag undo failed")
	}
}
func TestDesktopSourceScrollAndInvalidTab(t *testing.T) {
	g := testEditor(t)
	g.tab = tabPKG
	g.s.Project.PKG.Components = []zapp.Component{{ID: "app"}}
	g.rebuild()
	p := widgetProbe(t, g)
	f, _ := p.FindRole(ggui.RoleTextField, "Component ID")
	p.Scroll(f.Center(), ggui.Pt(0.0, -4.0))
	offset := g.desktop.offset("form:2:false:0").Get()
	if offset == 0 {
		t.Fatal("native scroll inactive")
	}
	p.Tap("JSON")
	p.Tap("Form")
	if g.desktop.offset("form:2:false:0").Get() != offset {
		t.Fatal("source switch lost scroll")
	}
	p.Tap("JSON")
	setTextField(t, p, "Components", "[")
	p.Tap("Project")
	if g.tab != tabPKG || g.input.Text() != "[" {
		t.Fatal("invalid draft lost while changing tabs")
	}
}
func TestDesktopIdleAndValueEditsRetainFieldModels(t *testing.T) {
	g := testEditor(t)
	g.tab = tabProject
	g.rebuild()
	p := widgetProbe(t, g)
	original := append([]*desktopField(nil), g.desktop.Fields.Get()...)
	g.desktop.sync()
	setTextField(t, p, "Output directory", "retained")
	for i, field := range original {
		if field != g.desktop.Fields.Get()[i] {
			t.Fatal("value edit replaced field state")
		}
	}
}

func closeProbe(t *testing.T, g *editor) *ggui.Probe {
	t.Helper()
	m := newDesktopModel(g)
	p := ggui.ProbeBuilder(func() ggui.Widget { return desktopView(m) }, ggui.Sz(g.w, g.h))
	m.dialogs = p.Dialogs()
	p.Setup(func() { uitheme.Set(editorTheme(true)) })
	p.OnCloseRequest(m.allowClose)
	t.Cleanup(p.Close)
	p.Frame()
	return p
}
func TestCloseRequestClosesCleanProject(t *testing.T) {
	g := testEditor(t)
	if !closeProbe(t, g).RequestClose() {
		t.Fatal("a project without edits should close at once")
	}
}
func TestCloseRequestAsksBeforeDiscardingEdits(t *testing.T) {
	g := testEditor(t)
	g.tab = tabProject
	g.rebuild()
	p := closeProbe(t, g)
	tapTextField(t, p, "Output directory")
	setTextField(t, p, "Output directory", "unsaved")
	if p.RequestClose() {
		t.Fatal("unsaved edits were closed without asking")
	}
	if !g.confirmClose {
		t.Fatal("close request did not open the confirm dialog")
	}
	p.Frame()
	p.Tap("Discard changes")
	if !g.quit {
		t.Fatal("discarding did not end the app")
	}
}
func TestCloseRequestCancelsRunningBuild(t *testing.T) {
	g := testEditor(t)
	cancelled := false
	g.build = &buildJob{cancel: func() { cancelled = true }}
	if closeProbe(t, g).RequestClose() {
		t.Fatal("closed while a build was running")
	}
	if !g.build.closeRequested || !cancelled {
		t.Fatalf("build was not cancelled: requested=%v cancelled=%v", g.build.closeRequested, cancelled)
	}
}
