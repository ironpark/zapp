package gui

import (
	"context"
	"fmt"
	"image/color"

	"github.com/ironpark/ggui"
	_ "github.com/ironpark/ggui/inspect/panel"
	uitheme "github.com/ironpark/ggui/ui/theme"
)

func editorTheme(dark bool) uitheme.Theme {
	preset := uitheme.Preset{Base: uitheme.BaseNeutral, Accent: uitheme.AccentBlue}
	t := preset.Light()
	if dark {
		t = preset.Dark()
		t.Primary = color.NRGBA{R: 139, G: 165, B: 255, A: 255}
		t.PrimaryFg = color.NRGBA{R: 20, G: 27, B: 52, A: 255}
		t.Ring = t.Primary
		t.PrimaryHover = t.Primary
	}
	t.Radius, t.RadiusSm, t.RadiusLg = 4, 0, 6
	t.Text.Size, t.Caption.Size, t.Title.Size = 13, 12, 19
	t.ButtonPad, t.FieldPad = ggui.Insets(6, 10), ggui.Insets(6, 10)
	return t
}
func (g *editor) runWidgets() error {
	m := newDesktopModel(g)
	app := ggui.New(ggui.Config{Title: "Zapp — Project settings", Width: g.w, Height: g.h, Resizable: true, Inspector: "f1", Accessibility: ggui.AccessibilityAlways}, func() ggui.Widget { return desktopView(m) })
	m.dialogs = app.Dialogs()
	app.Setup(func() { uitheme.Bind(m.Dark, editorTheme(true), editorTheme(false)) })
	ready := func(fn func()) func() {
		return g.action(func() {
			if g.build == nil && !g.confirmClose && g.picking == nil {
				fn()
			}
		})
	}
	// "cmd" is ggui's platform command modifier: Meta on macOS, Ctrl elsewhere.
	app.Shortcut("cmd+s", ready(func() { g.save() }))
	app.Shortcut("cmd+b", ready(g.startBuild))
	app.Shortcut("cmd+shift+v", ready(g.validate))
	app.OnDrop(func(e ggui.DropEvent) { ready(func() { g.dropFiles(e.Paths()) })() })
	app.OnKey(func(e ggui.KeyEvent) bool {
		if e.Kind == ggui.KeyPress && e.Mods.Cmd() && e.Key == ggui.KeyZ && g.active < 0 {
			ready(func() { g.history(e.Mods.Shift) })()
			return true
		}
		return false
	})
	for i := range sections {
		// ggui names number keys after their Ebitengine key name, so the chord
		// for the "1" key is "digit1"; a bare "1" is not a key it knows.
		app.Shortcut(fmt.Sprintf("cmd+digit%d", i+1), ready(func() { m.selectTab(i) }))
	}
	var runErr error
	// pump applies whatever background work has finished and acts on quit or
	// cancellation. It runs as posted work on the UI thread whenever a
	// goroutine, requestQuit or the context asks, instead of in an OnFrame
	// handler: ggui keeps drawing every frame while any OnFrame handler is
	// registered, so this is what lets an idle window stop painting.
	pump := func() {
		if err := g.ctx.Err(); err != nil {
			runErr = err
			app.Close()
			return
		}
		if g.quit {
			app.Close()
			return
		}
		g.pollAppIcons()
		g.pollBuild()
		g.pollSigning()
		if g.picking != nil {
			g.pollPicker()
		}
		m.sync()
	}
	g.wake = func() { app.Post(pump) }
	app.OnCloseRequest(m.allowClose)
	stopWake := context.AfterFunc(g.ctx, g.wake)
	defer stopWake()
	err := app.Run()
	if runErr != nil {
		return runErr
	}
	return err
}
