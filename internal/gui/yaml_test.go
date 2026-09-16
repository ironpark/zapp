package gui

import (
	"image"
	"strings"
	"testing"

	"github.com/ironpark/zapp"
)

func TestSegmentedWorkspaceBounds(t *testing.T) {
	for _, size := range []image.Point{{1080, 720}, {1200, 840}, {1600, 1000}} {
		g := testEditor(t)
		g.tab = tabDMG
		g.w, g.h = size.X, size.Y
		g.rebuild()
		segments := g.segmentedControls()
		if len(segments) != 2 {
			t.Fatal("missing segmented controls")
		}
		panels := []image.Rectangle{g.previewPanel().Bounds, g.settingsPanel().Bounds}
		for i, segment := range segments {
			if !segment.Bounds.In(panels[i]) {
				t.Fatalf("%v: segment outside panel", size)
			}
			for _, button := range g.controls() {
				if button.Bounds.Overlaps(segment.Bounds) {
					t.Fatalf("%v: segment overlaps %s", size, button.Label)
				}
			}
		}
		g.switchLayoutView(1)
		if !g.form.FieldBounds(0).In(g.form.Bounds) {
			t.Fatalf("%v: YAML editor outside form", size)
		}
		if g.form.Limit() != 0 {
			t.Fatalf("%v: YAML panel scrolls instead of its text", size)
		}
	}
}

func TestYAMLViewRoundTripAndUndo(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.s.Project.DMG.Contents = map[string]zapp.Content{}
	g.s.Project.DMG.Window.Width = 640
	g.rebuild()
	g.switchLayoutView(1)
	if !g.dmgYAML || g.fields[0].Syntax != "yaml" {
		t.Fatal("YAML view missing")
	}
	value := g.fields[0].Value
	if !strings.Contains(value, "contents: {}") || strings.Contains(value, "640.0") {
		t.Fatalf("serialization changed semantics: %s", value)
	}
	g.focus(0)
	g.input.SetText(strings.Replace(value, "width: 640", "width: 800", 1))
	if !g.commit() || g.s.Project.DMG.Window.Width != 800 || g.s.Project.DMG.Contents == nil {
		t.Fatalf("YAML edit not applied correctly: width=%d contents=%#v status=%s draft=%s", g.s.Project.DMG.Window.Width, g.s.Project.DMG.Contents, g.status, g.input.Text())
	}
	g.switchLayoutView(0)
	if g.dmgYAML || g.fields[0].Label != "Title" {
		t.Fatal("form view not restored")
	}
	g.history(false)
	if g.s.Project.DMG.Window.Width != 640 {
		t.Fatal("YAML transaction cannot undo")
	}
}

func TestInvalidYAMLRetainsDraftAndMode(t *testing.T) {
	for _, draft := range []string{"window: [", "window: {width: -1}", "unknown: value", "title: a\ntitle: b", "title: a\n---\ntitle: b", "null"} {
		g := testEditor(t)
		g.tab = tabDMG
		g.rebuild()
		g.switchLayoutView(1)
		g.focus(0)
		g.input.SetText(draft)
		g.switchLayoutView(0)
		if !g.dmgYAML || g.input.Text() != draft || g.input.Spec.Error == "" {
			t.Fatalf("invalid draft not retained: %q", draft)
		}
		if g.s.CanUndo() {
			t.Fatal("invalid draft entered history")
		}
	}
}

func TestLayoutViewSwitchDoesNotCreateChanges(t *testing.T) {
	g := testEditor(t)
	g.tab = tabDMG
	g.rebuild()
	for range 3 {
		g.switchLayoutView(1)
		g.focus(0)
		g.switchLayoutView(0)
	}
	if g.s.CanUndo() || g.s.Dirty() {
		t.Fatal("view switching changed project")
	}
}

func TestLayoutYAMLPosition(t *testing.T) {
	c := &zapp.DMGConfig{Contents: map[string]zapp.Content{"item": {Pos: &zapp.Position{180, 200}}}}
	text := layoutYAML(c)
	if !strings.Contains(text, "pos: [180, 200]") {
		t.Fatalf("expected inline position:\n%s", text)
	}
}
