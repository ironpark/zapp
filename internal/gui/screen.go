package gui

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/ironpark/zapp"
	"github.com/ironpark/zapp/internal/gui/comp"
)

func (g *editor) Draw(dst *ebiten.Image) {
	p := g.ui
	t := p.Theme
	mx, my := comp.PointerPosition(g.w, g.h)
	pointer := image.Pt(mx, my)
	dst.Fill(t.Background)
	p.Text(dst, "ZAPP", 24, 19, 26, t.Accent)
	state := "Saved"
	if !g.s.exists {
		state = "New project"
	}
	if g.dirty() {
		state = "Unsaved changes"
	}
	comp.Badge{Bounds: comp.Box(118, 25, 150, 28), Label: state, Highlight: g.dirty()}.Draw(dst, p)
	p.Text(dst, p.Fit(g.s.Path, g.w-48, 12), 24, 62, 12, t.Muted)
	// The toolbar and panels share the selected tab's continuous surface.
	comp.Rect(dst, comp.Box(24, 127, g.w-48, g.h-217), t.Panel)
	comp.Border(dst, comp.Box(24, 127, g.w-48, g.h-217), t.Border)
	g.tabs().Draw(dst, p, pointer)
	if g.tab == 0 {
		p.Text(dst, "Project workspace", 40, 148, 22, t.Text)
	}
	if !g.enabled() {
		comp.Panel{Bounds: comp.Box(40, 195, g.w-80, 180), Title: tabNames[g.tab] + " is disabled", Description: "Enable this step above to include it in your project."}.Draw(dst, p)
		p.Text(dst, "Your settings are retained while you work. Disabled steps are omitted from the saved configuration.", 56, 285, 14, t.Muted)
	} else {
		g.settingsPanel().Draw(dst, p)
		g.form.Draw(dst, p, g.active, &g.input)
		if g.tab == 1 {
			g.drawPreview(dst)
		} else {
			g.drawHelp(dst)
		}
	}
	comp.Rect(dst, comp.Box(0, g.h-76, g.w, 76), t.Panel)
	statusColor := t.Muted
	if g.failed {
		statusColor = t.Error
	}
	p.Wrapped(dst, g.status, 24, g.h-62, g.w-200, 13, statusColor, 2)
	p.Text(dst, "Ctrl/Cmd+S Save  ·  Ctrl/Cmd+1–6 Tabs  ·  Tab Next field  ·  Esc Cancel edit", 24, g.h-23, 11, t.Muted)
	// Every control is drawn exactly once, after its panel background.
	for _, button := range g.controls() {
		button.Draw(dst, p, pointer)
	}
	if g.tab > 0 {
		g.stepToggle().Draw(dst, p, pointer)
	}
	g.closeDialog().Draw(dst, p, pointer)
}

func (g *editor) drawHelp(dst *ebiten.Image) {
	x := g.settingsPanel().Bounds.Max.X + 16
	panel := comp.Panel{Bounds: comp.Box(x, 195, g.w-x-40, g.h-301), Title: "About this step"}
	panel.Draw(dst, g.ui)
	area := panel.Content()
	descriptions := []string{
		"Choose the app bundle and output directory shared by your packaging steps.",
		"Arrange the installer window and its contents.",
		"Configure the installer identity, destination and package contents.",
		"Bundle the libraries your app needs and configure dependency search paths.",
		"Choose the signing identity and entitlements for your app and installers.",
		"Configure Apple notarization and ticket stapling for distribution.",
	}
	g.ui.Wrapped(dst, descriptions[g.tab], area.Min.X, area.Min.Y, area.Dx(), 15, g.ui.Theme.Text, 4)
	y := area.Min.Y + 110
	sections := [][2]string{
		{"PATHS & VALUES", "Paths are relative to this configuration. ${env:NAME} expressions are preserved."},
		{"SAVE & VALIDATE", "Save writes all enabled steps. Validate checks build inputs. Run the CLI to build."},
		{"EDITING", "Tab moves to the next field. Ctrl/Cmd+Enter applies JSON. Esc cancels an edit."},
	}
	if g.tab == 4 || g.tab == 5 {
		sections[0] = [2]string{"CREDENTIALS", "Use environment variables for passwords and other sensitive values."}
	}
	for _, section := range sections {
		if y+90 > area.Max.Y {
			break
		}
		g.ui.Text(dst, section[0], area.Min.X, y, 11, g.ui.Theme.Accent)
		g.ui.Wrapped(dst, section[1], area.Min.X, y+25, area.Dx(), 13, g.ui.Theme.Muted, 3)
		y += 110
	}
}

func (g *editor) addFile() {
	key := strings.TrimSpace(g.newPath)
	if key == "" {
		g.adding = true
		g.form.ScrollTo(0)
		g.rebuild()
		g.focus(0)
		g.report(nil, "Enter a file or folder path, then click Add file again.")
		return
	}
	path := key
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(g.s.Path), path)
	}
	if _, err := os.Stat(path); err != nil {
		g.report(err, "")
		return
	}
	_, _, _, _, items := layout(g.s.Project.DMG, g.s.Project.App)
	for _, i := range items {
		if i.Path == key {
			g.report(fmt.Errorf("that path is already in the layout"), "")
			return
		}
	}
	g.s.checkpoint()
	g.s.materialize()
	w, h, _, _, _ := layout(g.s.Project.DMG, g.s.Project.App)
	x, y := w/2, h/2
	g.s.Project.DMG.Contents[key] = zapp.Content{X: &x, Y: &y}
	g.selected = key
	g.newPath = ""
	g.adding = false
	g.form.ScrollTo(0)
	g.rebuild()
	g.report(nil, "Added "+strconv.Quote(key)+". Drag its icon to position it.")
}
