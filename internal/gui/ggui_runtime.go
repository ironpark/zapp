package gui

import (
	"fmt"
	"image/color"
	"runtime"

	"github.com/hajimehoshi/ebiten/v2"
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
		app.Shortcut(fmt.Sprintf("%s%d", prefix, i+1), ready(func() { m.selectTab(i) }))
	}
	var runErr error
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
		if ebiten.IsWindowBeingClosed() {
			if g.build != nil {
				g.build.closeRequested = true
				if g.build.finished {
					g.finishClose()
				} else if !g.build.cancelling {
					g.dismissBuild()
				}
			} else if g.dirty() {
				g.confirmClose = true
			} else {
				g.quit = true
			}
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
