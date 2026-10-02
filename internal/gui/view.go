package gui

import (
	"fmt"
	"image/color"
	"path/filepath"

	"github.com/ironpark/ggui"
	"github.com/ironpark/ggui/ui"
	"github.com/ironpark/ggui/ui/icons"
	"github.com/ironpark/ggui/ui/icons/lucide"
	uitheme "github.com/ironpark/ggui/ui/theme"
)

func desktopView(m *desktopModel) ggui.Widget {
	return ggui.Reactive(func() ggui.Widget {
		t := uitheme.Use()
		g := m.editor
		toolbar := ggui.Padding(ggui.Row(
			ggui.Box(ggui.Text("Z").Color(t.PrimaryFg)).Fill(t.Primary).Radius(4).Pad(5, 9),
			ggui.Expanded(ggui.Column(ggui.TextOf(m.Title).Size(15).NoWrap().Ellipsis(), ui.Caption(filepath.Join(g.s.Dir, g.s.Name)).NoWrap().Ellipsis()).Gap(3)),
			ggui.TextOf(m.SaveState).Size(12).Color(t.MutedFg),
			// Undo and Redo leave their keys to a focused text field, which has
			// its own history, so the runtime's OnKey handles them, not Shortcut.
			ui.Tooltip(ghostIcon(lucide.Icon("undo-2"), "Undo", g.action(g.guard(func() { g.history(false) }))).BindDisabled(m.CanUndo.Map(func(v bool) bool { return !v })), "Undo").Shortcut("cmd+z"),
			ui.Tooltip(ghostIcon(lucide.Icon("redo-2"), "Redo", g.action(g.guard(func() { g.history(true) }))).BindDisabled(m.CanRedo.Map(func(v bool) bool { return !v })), "Redo").Shortcut("cmd+shift+z"),
			healthBadge(m),
			// A button's shortcut works while it is enabled and no dialog is
			// open over it, which is when the button itself would.
			ui.Tooltip(ui.Button("Save", g.action(func() { g.save() })).Shortcut("cmd+s").Outline().BindDisabled(m.Busy), "Save"),
			ui.Tooltip(ui.Button("Build", g.action(g.startBuild)).Shortcut("cmd+b").BindDisabled(m.Busy), "Build"),
			ggui.View(m.Dark, func(dark bool) ggui.Widget {
				icon, label := "sun", "Light appearance"
				if !dark {
					icon, label = "moon", "Dark appearance"
				}
				return iconButton(icon, label, func() { m.Dark.Set(!dark) })
			}),
		).Gap(10).Align(ggui.AlignCenter), 10, 16)
		footer := ggui.View(ggui.Combine(m.Status, m.Failed, func(message string, failed bool) statusView { return statusView{message, failed} }), func(v statusView) ggui.Widget {
			color := t.MutedFg
			if v.failed {
				color = t.Destructive
			}
			return ggui.Padding(ggui.Row(ggui.Text("●").Color(color), ggui.Expanded(ui.Tooltip(ggui.Text(v.message).Size(12).Color(color).NoWrap().Ellipsis(), v.message)), ggui.If(m.IssueTab.Map(func(tab int) bool { return tab >= 0 }), func() ggui.Widget { return ui.Button("Go to issue", g.action(g.goToIssue)).Ghost() })).Gap(8), 5, 16)
		})
		// Selecting a DMG item rebuilds only the item details, which follow
		// Selected themselves, not the whole workspace.
		page := ggui.ViewOf(m.Workspace, func(v workspaceState) workspaceState { v.Selected = ""; return v }, func(v workspaceState) ggui.Widget { return workspaceView(m, v) })
		body := ggui.Column(toolbar, ui.Divider(), stepTabs(m), ggui.Expanded(page), ui.Divider(), footer).Align(ggui.AlignStretch)
		return ggui.Column(ggui.Expanded(body), closeDialogView(m), ggui.ViewOf(m.Modal, buildShape, func(v buildDialogShape) ggui.Widget { return buildDialogView(m, v) })).Align(ggui.AlignStretch)
	})
}

// readyColor marks a project that passes its checks. The theme has no
// success color of its own, so one is picked to suit its background.
func readyColor(t uitheme.Theme) color.Color {
	r, g, b, _ := t.Bg.RGBA()
	if r+g+b < 3*0x8000 {
		return color.NRGBA{R: 74, G: 222, B: 128, A: 255}
	}
	return color.NRGBA{R: 21, G: 128, B: 61, A: 255}
}

// healthBadge reports whether the project is ready to build. Clicking it runs
// Validate, which moves to the first problem.
func healthBadge(m *desktopModel) ggui.Widget {
	g := m.editor
	return ggui.View(m.Health.Map(func(h projectHealth) int { return h.Count }), func(count int) ggui.Widget {
		t := uitheme.Use()
		icon, col := statusIcon(t, count == 0)
		label, tip := "Ready", "Build inputs look complete"
		if count > 0 {
			label, tip = fmt.Sprintf("%d issue%s", count, plural(count)), "Show the first issue"
		}
		content := ggui.Row(lucide.Icon(icon).Size(14).Color(col), ggui.Text(label).Color(col)).Gap(6).Align(ggui.AlignCenter)
		return ui.Tooltip(ui.ButtonOf(content, g.action(g.validate)).Name("Validate").Shortcut("cmd+shift+v").Ghost().BindDisabled(m.Busy), tip)
	})
}

// statusIcon is the icon and color marking a check that passed or failed.
func statusIcon(t uitheme.Theme, ok bool) (string, color.Color) {
	if ok {
		return "check", readyColor(t)
	}
	return "circle-alert", t.Destructive
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// stepTabs is the tab strip. Each tab shows its step's state: a disabled
// step is dimmed and a step with a problem carries a dot, and its tooltip
// names the problem. The pages are below it, in the workspace, so the
// tabs' own are empty.
func stepTabs(m *desktopModel) ggui.Widget {
	selected := ggui.Bind(m.Tab.Get, m.selectTab)
	return ggui.Reactive(func() ggui.Widget {
		t := uitheme.Use()
		health, enabled := m.Health.Get(), m.Enabled.Get()
		pages := []ui.TabPage{}
		for i, s := range sections {
			page := ui.Tab(s.Name, ggui.Box())
			if issue := health.Issues[i]; issue != "" || !enabled[i] {
				label := ggui.Text(s.Name).NoWrap()
				if !enabled[i] {
					label.Color(fade(t.MutedFg, .55))
				}
				head := []ggui.Widget{label}
				if issue != "" {
					head = append(head, ggui.Box().Size(6, 6).Radius(3).Fill(t.Destructive))
				}
				page = page.Header(ggui.Row(head...).Gap(6).Align(ggui.AlignCenter))
			}
			switch {
			case health.Issues[i] != "":
				page = page.Tooltip(health.Issues[i])
			case !enabled[i]:
				page = page.Tooltip(s.Name + " is off")
			}
			pages = append(pages, page)
		}
		return ui.Tabs(selected, pages...).Line()
	})
}

// fade scales a color's opacity.
func fade(c color.Color, amount float64) color.Color {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	n.A = uint8(float64(n.A) * amount)
	return n
}

type statusView struct {
	message string
	failed  bool
}

func closeDialogView(m *desktopModel) ggui.Widget {
	g := m.editor
	open := ggui.Bind(m.Close.Get, func(v bool) { g.confirmClose = v; m.sync() })
	return ui.Dialog(open, ggui.Column(ggui.Text("Your project has unsaved edits."), ggui.Row(ui.Button("Keep editing", func() { g.confirmClose = false; m.sync() }).Ghost(), ui.Button("Discard changes", func() { g.requestQuit() }).Outline(), ui.Button("Save & close", g.action(func() {
		if g.save() {
			g.requestQuit()
		} else {
			g.confirmClose = false
		}
	}))).Gap(8).Justify(ggui.JustifyEnd)).Gap(20)).Title("Save changes before closing?").Width(580)
}

// buildDialogShape is what the build dialog is built from: its log and
// message change with every line, so they are bound as text instead and
// only whether there is a log rebuilds it.
type buildDialogShape struct {
	modalState
	hasLog bool
}

func buildShape(v modalState) buildDialogShape {
	shape := buildDialogShape{v, v.Log != ""}
	shape.Log, shape.Message = "", ""
	return shape
}
func buildDialogView(m *desktopModel, v buildDialogShape) ggui.Widget {
	if v.Title == "" {
		return ggui.Box()
	}
	g := m.editor
	actions := []ggui.Widget{}
	body := []ggui.Widget{ggui.TextOf(m.Modal.Map(func(v modalState) string { return v.Message }))}
	if v.hasLog {
		t := uitheme.Use()
		// The log follows new lines until the user scrolls up, and again once
		// they scroll back to the end.
		log := ggui.Scroll(ggui.Padding(ggui.TextOf(m.Modal.Map(func(v modalState) string { return v.Log })).Style(ggui.TextStyle{Font: ggui.DefaultMonoFont(), Size: 12}).Color(t.MutedFg), 10, 12)).BindOffset(m.offset("build-log")).FollowEnd()
		body = append(body, ggui.Box(log).Height(220).Fill(t.Muted).Radius(t.Radius))
		actions = append(actions, ui.Button("Copy log", func() { ggui.CurrentClipboard().Write(ggui.Untrack(m.Modal.Get).Log) }).Ghost(), ggui.Spacer())
	}
	if v.Reveal != "" {
		actions = append(actions, ui.Button(revealLabel(), func() { g.reveal(v.Reveal) }).Outline())
	}
	label := "Cancel build"
	if v.Finished {
		label = "Close"
	}
	actions = append(actions, ui.Button(label, g.action(g.dismissBuild)).Outline().Disabled(v.Cancelling && !v.Finished))
	if v.Finished && v.Issue {
		actions = append(actions, ui.Button("Go to issue", g.action(func() { g.dismissBuild(); g.goToIssue() })))
	}
	open := ggui.Bind(func() bool { return m.Modal.Get().Title != "" }, func(b bool) {
		if !b && g.build != nil {
			g.dismissBuild()
			m.sync()
		}
	})
	body = append(body, ggui.Row(actions...).Gap(8).Justify(ggui.JustifyEnd))
	return ui.Dialog(open, ggui.Column(body...).Gap(16).Align(ggui.AlignStretch)).Title(v.Title).Width(680)
}

// scrollGutter keeps scrolled content clear of the overlay scrollbar drawn
// along the right edge.
const scrollGutter = 12

// Keep icon creation close to its named control so accessibility has a textual
// action even when the visual label is only an icon.
func iconButton(icon, name string, action func()) ggui.Widget {
	return ui.Tooltip(ghostIcon(lucide.Icon(icon), name, action), name)
}

// ghostIcon is the borderless button behind every icon-only control.
func ghostIcon(icon *icons.Widget, name string, action func()) *ui.ButtonWidget {
	return ui.ButtonOf(icon.Size(16), action).Name(name).Ghost().Pad(6)
}
