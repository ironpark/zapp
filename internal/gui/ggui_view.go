package gui

import (
	"fmt"
	"slices"

	"github.com/ironpark/ggui"
	"github.com/ironpark/ggui/ui"
	"github.com/ironpark/ggui/ui/icons/lucide"
	uitheme "github.com/ironpark/ggui/ui/theme"
)

func desktopView(m *desktopModel) ggui.Widget {
	return ggui.Reactive(func() ggui.Widget {
		t := uitheme.Use()
		g := m.editor
		toolbar := ggui.Padding(ggui.Row(
			ggui.Box(ggui.Text("Z").Color(t.PrimaryFg)).Fill(t.Primary).Radius(4).Pad(5, 9),
			ggui.Text("Zapp").Size(18),
			ggui.Expanded(ggui.Column(ggui.Text(g.s.Name).NoWrap(), ui.Caption(g.s.Dir).NoWrap()).Gap(3)),
			ggui.TextOf(m.SaveState).Size(12).Color(t.MutedFg),
			ui.Tooltip(ui.Button("↶", g.action(g.guard(func() { g.history(false) }))).Name("Undo").Ghost().BindDisabled(m.CanUndo.Map(func(v bool) bool { return !v })), "Undo · Ctrl/Cmd+Z"),
			ui.Tooltip(ui.Button("↷", g.action(g.guard(func() { g.history(true) }))).Name("Redo").Ghost().BindDisabled(m.CanRedo.Map(func(v bool) bool { return !v })), "Redo · Ctrl/Cmd+Shift+Z"),
			ui.Button("Validate", g.action(g.validate)).Ghost().BindDisabled(m.Busy),
			ui.Button("Save", g.action(func() { g.save() })).Outline().BindDisabled(m.Busy),
			ui.Button("Build", g.action(g.startBuild)).BindDisabled(m.Busy),
			ui.ThemeSwitch(m.Dark),
		).Gap(10).Align(ggui.AlignCenter), 10, 16)
		tabs := []ui.TabPage{}
		for i, s := range sections {
			tabs = append(tabs, ui.Tab(s.Name, ggui.View(m.Workspace, func(v workspaceState) ggui.Widget {
				if v.Tab != i {
					return ggui.Box()
				}
				return workspaceView(m, v)
			})))
		}
		tabTheme := t
		tabTheme.Radius = 0
		tabTheme.RadiusSm = 0
		pages := uitheme.With(tabTheme, ui.Tabs(ggui.Bind(m.Tab.Get, m.selectTab), tabs...).Line())
		footer := ggui.View(ggui.Combine(m.Status, m.Failed, func(message string, failed bool) statusView { return statusView{message, failed} }), func(v statusView) ggui.Widget {
			color := t.MutedFg
			if v.failed {
				color = t.Destructive
			}
			return ggui.Padding(ggui.Row(ggui.Text("●").Color(color), ggui.Expanded(ui.Tooltip(ggui.Text(v.message).Size(12).Color(color).NoWrap(), v.message)), ggui.If(m.Workspace.Map(func(v workspaceState) bool { return v.IssueTab >= 0 }), func() ggui.Widget { return ui.Button("Go to issue", g.action(g.goToIssue)).Ghost() })).Gap(8), 5, 16)
		})
		body := ggui.Column(toolbar, ui.Divider(), ggui.Expanded(pages), ui.Divider(), footer).Align(ggui.AlignStretch)
		return ggui.Column(ggui.Expanded(body), closeDialogView(m), ggui.View(m.Modal, func(v modalState) ggui.Widget { return buildDialogView(m, v) })).Align(ggui.AlignStretch)
	})
}

type statusView struct {
	message string
	failed  bool
}

func workspaceView(m *desktopModel, v workspaceState) ggui.Widget {
	g := m.editor
	status := "Disabled"
	if v.Enabled {
		status = "Enabled"
	}
	actions := []ggui.Widget{ggui.Column(ui.Title(sections[v.Tab].Name), ui.Caption(sections[v.Tab].Description)).Gap(4), ggui.Spacer()}
	if v.Tab == tabPKG && v.Enabled {
		actions = append(actions, modeButtons([]string{"Single app", "Components"}, boolIndex(v.Full), func(i int) {
			if (i == 1) != v.Full {
				g.guard(g.switchPackageForm)()
				m.sync()
			}
		}))
	}
	if v.Tab != tabDMG && v.Enabled {
		actions = append(actions, ui.Button("Help", func() { g.helpOpen = !g.helpOpen; m.sync() }).Ghost())
	}
	if sections[v.Tab].Optional() {
		actions = append(actions, ui.Switch(ggui.Bind(func() bool { return m.Workspace.Get().Enabled }, func(bool) { g.guard(g.toggle)(); m.sync() }), status).Name("Enable "+sections[v.Tab].Name))
	}
	header := ggui.Row(actions...).Gap(12).Align(ggui.AlignCenter)
	var body ggui.Widget
	if !v.Enabled {
		body = ggui.Center(ui.Empty(sections[v.Tab].Name+" is disabled", "Enable this step to configure it. Settings are retained during this session."))
	} else if v.Tab == tabDMG {
		body = designerView(m, v)
	} else {
		panels := []ggui.Widget{}
		if v.Full {
			panels = append(panels, componentSidebar(m, v))
		}
		panels = append(panels, ggui.Expanded(settingsView(m, v)), ggui.If(m.Help, func() ggui.Widget { return helpView(v.Tab) }))
		body = ggui.Row(panels...).Gap(16).Align(ggui.AlignStretch)
	}
	return ggui.Padding(ggui.Column(header, ggui.Expanded(body)).Gap(16).Align(ggui.AlignStretch), 16)
}
func boolIndex(b bool) int {
	if b {
		return 1
	}
	return 0
}
func modeButtons(labels []string, index int, selectMode func(int)) ggui.Widget {
	out := []ggui.Widget{}
	for i, label := range labels {
		button := ui.Button(label, func() { selectMode(i) }).Pad(5, 10)
		if i == index {
			button.Secondary()
		} else {
			button.Ghost()
		}
		out = append(out, button)
	}
	return ggui.Row(out...).Gap(2)
}
func surface(content ggui.Widget) ggui.Widget {
	t := uitheme.Use()
	return ggui.Box(content).Fill(t.Card).Border(1, t.Border).Radius(t.RadiusLg).Pad(16)
}
func settingsView(m *desktopModel, v workspaceState) ggui.Widget {
	title := "Settings"
	if v.Tab == tabDMG {
		title = "Layout settings"
	}
	header := []ggui.Widget{ggui.Text(title), ggui.Spacer()}
	if v.Tab == tabDMG || v.Full || v.Tab == tabDep {
		labels := []string{"Form", "JSON"}
		if v.Tab == tabDMG {
			labels[1] = "YAML"
		}
		if v.Tab == tabDep {
			labels = []string{"List", "Text"}
		}
		header = append(header, modeButtons(labels, boolIndex(v.Raw), func(i int) { m.source(i == 1) }))
	}
	children := []ggui.Widget{ggui.Row(header...).Gap(8)}
	if v.Tab == tabSign {
		children = append(children, modeButtons([]string{"Keychain", "PKCS#12", "PEM"}, v.SignMethod, m.editor.actionSelect(m.editor.selectSignMethod)))
	}
	if v.Tab == tabNotarize {
		children = append(children, modeButtons([]string{"Profile", "Apple ID", "API key"}, v.NotaryMethod, m.editor.actionSelect(m.editor.selectNotaryMethod)))
	}
	key := fmt.Sprintf("form:%d:%v:%d", v.Tab, v.Raw, v.Component)
	children = append(children, ggui.Expanded(ggui.Scroll(formView(m, m.Fields)).Key(key).BindOffset(m.offset(key))))
	if v.Tab == tabPKG || (v.Tab == tabDMG && !v.Raw) {
		label := "Advanced settings"
		if v.Advanced {
			label = "Hide advanced settings"
		}
		children = append(children, ui.Divider(), ui.Button(label, m.advanced).Ghost())
	}
	if v.Tab == tabDep && !v.Raw {
		children = append(children, ui.Button("Add directory", m.editor.action(m.editor.guard(func() { m.editor.browse(len(m.editor.s.Project.Dep.Libs)) }))).Outline())
	}
	return surface(ggui.Column(children...).Gap(12).Align(ggui.AlignStretch))
}
func (g *editor) actionSelect(fn func(int)) func(int) { return func(i int) { fn(i); g.invalidate() } }
func componentSidebar(m *desktopModel, v workspaceState) ggui.Widget {
	g := m.editor
	rows := ggui.EachKeyed(m.Components, func(c componentRow) int { return c.Index }, func(row ggui.EachItem[componentRow]) ggui.Widget {
		return ggui.Reactive(func() ggui.Widget {
			c := row.Value.Get()
			state := m.Workspace.Get()
			b := ui.Button(c.Label, func() { m.selectComponent(c.Index) }).Name("Component "+c.Label).Pad(8, 10)
			if c.Index == state.Component && !state.Raw {
				b.Secondary()
			} else {
				b.Ghost()
			}
			return ggui.FromFuncs(b.Layout, func(dst *ggui.Canvas, r ggui.Rect) {
				dst.Paint(b, r)
				if m.revealComponent == c.Index {
					dst.RequestFocus(b)
					m.revealComponent = -1
				}
			})
		})
	}).Gap(4)
	content := ggui.Column(ggui.TextOf(m.Components.Map(func(c []componentRow) string { return fmt.Sprintf("Components · %d", len(c)) })), ui.Caption("Installer payloads"), ggui.Expanded(ggui.Scroll(rows)), ui.Button("Add component", g.action(g.guard(g.addComponent))).Outline(), ui.Button("Remove", g.action(g.guard(g.removeComponent))).Ghost().BindDisabled(ggui.Derived(func() bool { return m.Workspace.Get().Raw || len(m.Components.Get()) == 0 }))).Gap(10).Align(ggui.AlignStretch)
	return ggui.Box(surface(content)).Width(210)
}
func helpView(tab int) ggui.Widget {
	t := uitheme.Use()
	children := []ggui.Widget{ui.Title("About this step"), ui.Caption(sections[tab].Description)}
	for _, h := range stepHelp[tab] {
		children = append(children, ggui.Text(h[0]).Size(12).Color(t.Primary), ggui.Text(h[1]).Size(13))
	}
	return ggui.Box(surface(ggui.Scroll(ggui.Column(children...).Gap(16).Align(ggui.AlignStretch)))).Width(280)
}
func closeDialogView(m *desktopModel) ggui.Widget {
	g := m.editor
	open := ggui.Bind(m.Close.Get, func(v bool) { g.confirmClose = v; m.sync() })
	return ui.Dialog(open, ggui.Column(ggui.Text("Your project has unsaved edits."), ggui.Row(ui.Button("Keep editing", func() { g.confirmClose = false; m.sync() }).Ghost(), ui.Button("Discard changes", func() { g.quit = true }).Outline(), ui.Button("Save & close", g.action(func() {
		if g.save() {
			g.quit = true
		} else {
			g.confirmClose = false
		}
	}))).Gap(8).Justify(ggui.JustifyEnd)).Gap(20)).Title("Save changes before closing?").Width(580)
}
func buildDialogView(m *desktopModel, v modalState) ggui.Widget {
	if v.Title == "" {
		return ggui.Box()
	}
	g := m.editor
	actions := []ggui.Widget{}
	if v.Build {
		label := "Cancel build"
		if v.Finished {
			label = "Close"
		}
		actions = append(actions, ui.Button(label, g.action(g.dismissBuild)).Outline().Disabled(v.Cancelling && !v.Finished))
		if v.Finished && v.Issue {
			actions = append(actions, ui.Button("Go to issue", g.action(func() { g.dismissBuild(); g.goToIssue() })))
		}
	}
	open := ggui.Bind(func() bool { return m.Modal.Get().Title != "" }, func(b bool) {
		if !b && g.build != nil {
			g.dismissBuild()
			m.sync()
		}
	})
	return ui.Dialog(open, ggui.Column(ggui.Text(v.Message), ggui.Row(actions...).Gap(8).Justify(ggui.JustifyEnd)).Gap(20)).Title(v.Title).Width(600)
}
func removeLibrary(g *editor, i int) {
	if !g.commit() {
		return
	}
	g.s.checkpoint()
	g.s.Project.Dep.Libs = slices.Delete(g.s.Project.Dep.Libs, i, i+1)
	g.clearIssue(tabDep)
	g.rebuild()
}

// Keep icon creation close to its named control so accessibility has a textual
// action even when the visual label is only an icon.
func iconButton(icon, name string, action func()) ggui.Widget {
	return ui.Tooltip(ui.ButtonOf(lucide.Icon(icon).Size(16), action).Name(name).Ghost().Pad(6), name)
}
