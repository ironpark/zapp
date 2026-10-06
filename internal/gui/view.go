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

// desktopView is the window. It is built once: what changes is bound, or
// rebuilt by a View of its own, and colors are theme tokens, so a theme
// switch rebuilds nothing.
func desktopView(m *desktopModel) ggui.Widget {
	// Selecting a DMG item rebuilds only the item details, which follow
	// Selected themselves, not the whole workspace.
	page := ggui.ViewOf(m.Workspace,
		func(v workspaceState) workspaceState { v.Selected = ""; return v },
		func(v workspaceState) ggui.Widget { return workspaceView(m, v) })
	body := ggui.Column(toolbar(m), ui.Divider(), stepTabs(m), ggui.Expanded(page), ui.Divider(), statusLine(m)).
		Align(ggui.AlignStretch)
	buildDialog := ggui.ViewOf(m.Modal, buildShape, func(v buildDialogShape) ggui.Widget { return buildDialogView(m, v) })
	return ggui.Column(ggui.Expanded(body), closeDialogView(m), buildDialog).Align(ggui.AlignStretch)
}

// toolbar is the strip along the top: the project, its save state, history,
// health and the Save and Build actions.
func toolbar(m *desktopModel) ggui.Widget {
	g := m.editor
	logo := ggui.Box(ggui.Text("Z").Color(uitheme.PrimaryFg)).Fill(uitheme.Primary).Radius(4).Pad(5, 9)
	project := ggui.Column(
		ggui.TextOf(m.Title).Size(15).NoWrap().Ellipsis(),
		ui.Caption(filepath.Join(g.s.Dir, g.s.Name)).NoWrap().Ellipsis(),
	).Gap(3)
	// Undo and Redo leave their keys to a focused text field, which has its
	// own history, so the runtime's OnKey handles them, not Shortcut.
	undo := ghostIcon(lucide.Icon("undo-2"), "Undo", g.action(g.guard(func() { g.history(false) }))).BindDisabled(not(m.CanUndo))
	redo := ghostIcon(lucide.Icon("redo-2"), "Redo", g.action(g.guard(func() { g.history(true) }))).BindDisabled(not(m.CanRedo))
	// A button's shortcut works while it is enabled and no dialog is open over
	// it, which is when the button itself would.
	save := ui.Button("Save", g.action(func() { g.save() })).Shortcut("cmd+s").Outline().BindDisabled(m.Busy)
	build := ui.Button("Build", g.action(g.startBuild)).Shortcut("cmd+b").BindDisabled(m.Busy)
	appearance := ggui.View(m.Dark, func(dark bool) ggui.Widget {
		icon, label := "sun", "Light appearance"
		if !dark {
			icon, label = "moon", "Dark appearance"
		}
		return iconButton(icon, label, func() { m.Dark.Set(!dark) })
	})
	return ggui.Padding(ggui.Row(
		logo,
		ggui.Expanded(project),
		ggui.TextOf(m.SaveState).Size(12).Color(uitheme.MutedFg),
		ui.Tooltip(undo, "Undo").Shortcut("cmd+z"),
		ui.Tooltip(redo, "Redo").Shortcut("cmd+shift+z"),
		healthBadge(m),
		ui.Tooltip(save, "Save"),
		ui.Tooltip(build, "Build"),
		appearance,
	).Gap(10).Align(ggui.AlignCenter), 10, 16)
}

// statusLine is the strip along the bottom: the last report, and Go to issue
// while there is one.
func statusLine(m *desktopModel) ggui.Widget {
	g := m.editor
	status := ggui.Combine(m.Status, m.Failed, func(message string, failed bool) statusView { return statusView{message, failed} })
	return ggui.View(status, func(v statusView) ggui.Widget {
		color := uitheme.MutedFg
		if v.failed {
			color = uitheme.Destructive
		}
		message := ui.Tooltip(ggui.Text(v.message).Size(12).Color(color).NoWrap().Ellipsis(), v.message)
		goToIssue := ggui.If(m.IssueTab.Map(func(tab int) bool { return tab >= 0 }), func() ggui.Widget {
			return ui.Button("Go to issue", g.action(g.goToIssue)).Ghost()
		})
		return ggui.Padding(ggui.Row(ggui.Text("●").Color(color), ggui.Expanded(message), goToIssue).Gap(8), 5, 16)
	})
}

// not is the negation of r, for binding Disabled to a "can".
func not(r ggui.Readable[bool]) ggui.Readable[bool] {
	return ggui.Map(r, func(v bool) bool { return !v })
}

// success marks a check that passed. shadcn/ui leaves it to the app; the
// themes set it to suit their background.
var success = uitheme.Var("success")

// healthBadge reports whether the project is ready to build. Clicking it runs
// Validate, which moves to the first problem.
func healthBadge(m *desktopModel) ggui.Widget {
	g := m.editor
	return ggui.View(m.Health.Map(func(h projectHealth) int { return h.Count }), func(count int) ggui.Widget {
		icon, col := statusIcon(count == 0)
		label, tip := "Ready", "Build inputs look complete"
		if count > 0 {
			label, tip = fmt.Sprintf("%d issue%s", count, plural(count)), "Show the first issue"
		}
		content := ggui.Row(lucide.Icon(icon).Size(14).Color(col), ggui.Text(label).Color(col)).Gap(6).Align(ggui.AlignCenter)
		return ui.Tooltip(ui.ButtonOf(content, g.action(g.validate)).Name("Validate").Shortcut("cmd+shift+v").Ghost().BindDisabled(m.Busy), tip)
	})
}

// statusIcon is the icon and color marking a check that passed or failed.
func statusIcon(ok bool) (string, color.Color) {
	if ok {
		return "check", success
	}
	return "circle-alert", uitheme.Destructive
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
		health, enabled := m.Health.Get(), m.Enabled.Get()
		pages := []ui.TabPage{}
		for i, s := range sections {
			page := ui.Tab(s.Name, ggui.Box())
			if issue := health.Issues[i]; issue != "" || !enabled[i] {
				label := ggui.Text(s.Name).NoWrap()
				if !enabled[i] {
					label.Color(uitheme.MutedFg.Alpha(.55))
				}
				head := []ggui.Widget{label}
				if issue != "" {
					head = append(head, ggui.Box().Size(6, 6).Radius(3).Fill(uitheme.Destructive))
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

type statusView struct {
	message string
	failed  bool
}

func closeDialogView(m *desktopModel) ggui.Widget {
	g := m.editor
	keepEditing := func() { g.confirmClose = false; g.invalidate() }
	saveAndClose := g.action(func() {
		if g.save() {
			g.requestQuit()
		} else {
			g.confirmClose = false
		}
	})
	actions := ggui.Row(
		ui.Button("Keep editing", keepEditing).Ghost(),
		ui.Button("Discard changes", g.requestQuit).Outline(),
		ui.Button("Save & close", saveAndClose),
	).Gap(8).Justify(ggui.JustifyEnd)
	open := ggui.Bind(m.Close.Get, func(v bool) { g.confirmClose = v; g.invalidate() })
	return ui.Dialog(open, ggui.Column(ggui.Text("Your project has unsaved edits."), actions).Gap(20)).
		Title("Save changes before closing?").Width(580)
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
		// The log follows new lines until the user scrolls up, and again once
		// they scroll back to the end.
		text := ggui.TextOf(m.Modal.Map(func(v modalState) string { return v.Log })).
			Style(ggui.TextStyle{Font: ggui.DefaultMonoFont(), Size: 12}).Color(uitheme.MutedFg)
		log := ggui.Scroll(ggui.Padding(text, 10, 12)).BindOffset(m.offset("build-log")).FollowEnd()
		copyLog := func() { ggui.CurrentClipboard().Write(ggui.Untrack(m.Modal.Get).Log) }
		body = append(body, ggui.Box(log).Height(220).Fill(uitheme.Muted).Radius(ggui.Untrack(uitheme.Use).Radius))
		actions = append(actions, ui.Button("Copy log", copyLog).Ghost(), ggui.Spacer())
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
			g.invalidate()
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
