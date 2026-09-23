package gui

import (
	"context"
	"fmt"
	"image/color"
	"runtime"

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
	prefix := "ctrl+"
	if runtime.GOOS == "darwin" {
		prefix = "cmd+"
	}
	ready := func(fn func()) func() {
		return g.action(func() {
			if g.build == nil && !g.confirmClose && g.picking == nil {
				fn()
			}
		})
	}
	app.Shortcut(prefix+"s", ready(func() { g.save() }))
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
		app.Shortcut(fmt.Sprintf("%sdigit%d", prefix, i+1), ready(func() { m.selectTab(i) }))
	}
	// Anything that ends the app from inside a frame or a click has already
	// missed this frame's quit check, and ggui paints no idle frames, so it
	// asks for the next one.
	g.wake = func() { app.Post(func() {}) }
	app.OnCloseRequest(m.allowClose)
	var runErr error
	// Cancellation is only observed inside a frame, so wake the window when the
	// context ends. Post is safe from another goroutine.
	stopWake := context.AfterFunc(g.ctx, func() { app.Post(func() {}) })
	defer stopWake()
	app.OnFrame(func() {
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
		if g.picking != nil {
			g.pollPicker()
		}
		// ggui only schedules a frame while something is moving, but these
		// pollers drain channels that background goroutines fill. Keep asking
		// for frames while any of that work is outstanding, or a finished
		// build, an extracted icon or a chosen path would sit unnoticed until
		// the pointer happened to move.
		if g.build != nil || g.picking != nil || len(g.appIconPending) > 0 {
			app.Post(func() {})
		}
		m.sync()
		if m.focus != nil {
			if tree := app.Semantics(); tree != nil {
				for _, n := range tree.Nodes(ggui.RoleTextField) {
					if n.ID.ID == *m.focus {
						app.Perform(n.ID, ggui.Action{Kind: ggui.ActionFocus})
						m.focus = nil
						break
					}
				}
			}
		}
	})
	err := app.Run()
	if runErr != nil {
		return runErr
	}
	return err
}
