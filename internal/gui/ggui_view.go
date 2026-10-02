package gui

import (
	"fmt"
	"image/color"
	"path/filepath"
	"slices"
	"strings"

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

func workspaceView(m *desktopModel, v workspaceState) ggui.Widget {
	g := m.editor
	status := "Disabled"
	if v.Enabled {
		status = "Enabled"
	}
	actions := []ggui.Widget{ggui.Column(ui.Title(sections[v.Tab].Name), ui.Caption(sections[v.Tab].Description)).Gap(4), ggui.Spacer()}
	if v.Tab == tabPKG && v.Enabled {
		actions = append(actions, modeButtons("Package form", []string{"Single app", "Components"}, boolIndex(v.Full), func(int) {
			g.guard(g.switchPackageForm)()
			m.sync()
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

// modeButtons picks one of labels, the one at index now; selectMode gets a
// different choice and may refuse it. A choice taken comes back as a new
// index when the view rebuilds, and one refused leaves index where it was.
func modeButtons(name string, labels []string, index int, selectMode func(int)) ggui.Widget {
	options := make([]int, len(labels))
	for i := range options {
		options[i] = i
	}
	return ui.ToggleGroup(ggui.Controlled(index, selectMode)).Options(options).Format(func(i int) string { return labels[i] }).Name(name)
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
		header = append(header, modeButtons("Edit as", labels, boolIndex(v.Raw), func(i int) { m.source(i == 1) }))
	}
	children := []ggui.Widget{ggui.Row(header...).Gap(8)}
	g := m.editor
	if v.Tab == tabSign {
		check := m.Signing.Map(func(s signingState) signCheck { return s.Check })
		children = append(children, ggui.Row(modeButtons("Signing method", []string{"Keychain", "PKCS#12", "PEM"}, v.SignMethod, g.actionSelect(g.selectSignMethod)),
			ui.Tooltip(ui.Button("Check", g.action(g.checkSigning)).Outline().BindDisabled(check.Map(func(c signCheck) bool { return c.Checking })), "Find the certificate these settings sign with, without building")).Gap(16).Align(ggui.AlignCenter),
			ggui.If(check.Map(func(c signCheck) bool { return c.Shown }), func() ggui.Widget { return ggui.View(check, checkResult) }))
	}
	if v.Tab == tabNotarize {
		children = append(children, modeButtons("Notarization method", []string{"Profile", "Apple ID", "API key"}, v.NotaryMethod, g.actionSelect(g.selectNotaryMethod)))
	}
	var content ggui.Widget = formView(m, m.Fields)
	switch {
	case v.Tab == tabProject:
		content = ggui.Column(appCard(m), content, buildSteps(m)).Gap(28).Align(ggui.AlignStretch)
	case v.Tab == tabSign && v.SignMethod == 0:
		listed := m.Signing.Map(func(s signingState) bool { return s.IdentitiesListed })
		content = ggui.Column(content, ggui.If(listed, func() ggui.Widget {
			return ggui.View(m.Signing, func(s signingState) ggui.Widget { return identitySuggestions(m, s) })
		})).Gap(20).Align(ggui.AlignStretch)
	}
	if v.Tab != tabDMG && !v.Raw {
		content = ggui.Box(content).MaxWidth(formMaxWidth)
	}
	key := fmt.Sprintf("form:%d:%v:%d", v.Tab, v.Raw, v.Component)
	children = append(children, ggui.Expanded(ggui.Scroll(ggui.Padding(content, 0, scrollGutter, 0, 0)).Key(key).BindOffset(m.offset(key))))
	if v.Tab == tabPKG || (v.Tab == tabDMG && !v.Raw) {
		label := "Advanced settings"
		if v.Advanced {
			label = "Hide advanced settings"
		}
		children = append(children, ui.Divider(), ui.Button(label, m.advanced).Ghost())
	}
	return ui.Card(ggui.Column(children...).Gap(12).Align(ggui.AlignStretch))
}

// formMaxWidth keeps single forms readable in a wide window: labels, inputs
// and their Browse buttons stay within one glance.
const formMaxWidth = 760

// checkResult shows what the Signing tab's Check found.
func checkResult(c signCheck) ggui.Widget {
	t := uitheme.Use()
	if c.Checking {
		return ui.Caption("Checking the certificate…")
	}
	icon, col := statusIcon(t, c.OK)
	return ggui.Row(lucide.Icon(icon).Size(14).Color(col), ggui.Expanded(ggui.Text(c.Message).Size(12).Color(col))).Gap(6).Align(ggui.AlignStart)
}

// identitySuggestions offers the keychain's signing identities to fill the
// Keychain method's identity with one click.
func identitySuggestions(m *desktopModel, v signingState) ggui.Widget {
	g := m.editor
	refresh := ui.Tooltip(ui.Button("Refresh", g.action(g.refreshIdentities)).Ghost().Pad(2, 8).Disabled(v.IdentitiesListing),
		"Look up the keychain again, as after importing a certificate")
	children := []ggui.Widget{ggui.Row(ggui.Expanded(ggui.Text("In your keychain").Size(12).Color(uitheme.Use().Primary)), refresh).Align(ggui.AlignCenter)}
	if v.Identities == "" {
		children = append(children, ui.Caption("No valid signing identities were found. Import your Developer ID certificate, then refresh, or use the PKCS#12 or PEM method."))
	} else {
		for _, name := range strings.Split(v.Identities, "\n") {
			children = append(children, ui.ButtonOf(ggui.Row(ggui.Expanded(ggui.Text(name).NoWrap().Ellipsis())), g.action(func() { g.useIdentity(name) })).Name("Use "+name).Outline())
		}
		children = append(children, ui.Caption("Leave the identity blank to pick the first matching Developer ID automatically."))
	}
	return ggui.Column(children...).Gap(8).Align(ggui.AlignStretch)
}

// appCard introduces the Project tab with the app being packaged, as its
// Info.plist describes it, so a wrong bundle is obvious at once.
func appCard(m *desktopModel) ggui.Widget {
	g := m.editor
	// Reactive rather than a memo of the summary: the icon can arrive later
	// for the same app, and only Assets changes then.
	return ggui.Reactive(func() ggui.Widget {
		t := uitheme.Use()
		app := m.Health.Get().App
		m.Assets.Get()
		var icon ggui.Widget = ggui.Box().Size(48, 48).Radius(10).Fill(t.Muted)
		if img := g.assets[projectIconKey]; img != nil {
			icon = ggui.Image(img).Size(48, 48)
		}
		title, caption := app.Name, ggui.Widget(nil)
		switch {
		case app.Path == "":
			title, caption = "No app selected", ui.Caption("Choose the .app bundle below, or drop it anywhere in this window.")
		case app.Error != "":
			caption = ggui.Text(app.Error).Size(12).Color(t.Destructive)
		default:
			details := []string{}
			if app.Version != "" {
				details = append(details, "Version "+app.Version)
			}
			if app.BundleID != "" {
				details = append(details, app.BundleID)
			}
			caption = ui.Caption(strings.Join(details, " · "))
		}
		return ggui.Row(icon, ggui.Expanded(ggui.Column(ggui.Text(title).Size(16).NoWrap().Ellipsis(), caption).Gap(4))).Gap(14).Align(ggui.AlignCenter)
	})
}

// buildSteps lists what Build will do, in the order it does it, with each
// step's state and output. A row opens its tab.
func buildSteps(m *desktopModel) ggui.Widget {
	g := m.editor
	return ggui.Reactive(func() ggui.Widget {
		t := uitheme.Use()
		health, enabled := m.Health.Get(), m.Enabled.Get()
		rows := []ggui.Widget{ggui.Text("Build steps").Size(12).Color(t.Primary), ui.Divider()}
		for _, tab := range []int{tabDep, tabSign, tabDMG, tabPKG, tabNotarize, tabDistribution} {
			icon, col := statusIcon(t, health.Issues[tab] == "")
			detail, detailCol := g.stepDetail(tab, health), t.MutedFg
			switch {
			case !enabled[tab] || detail == "":
				icon, col, detail = "minus", t.MutedFg, "Off"
			case health.Issues[tab] != "":
				detail, detailCol = health.Issues[tab], t.Destructive
			}
			row := ggui.Row(lucide.Icon(icon).Size(16).Color(col), ggui.Expanded(ggui.Column(ggui.Text(sections[tab].Name), ggui.Text(detail).Size(12).Color(detailCol).NoWrap().Ellipsis()).Gap(3)), lucide.Icon("chevron-right").Size(14).Color(t.MutedFg)).Gap(12).Align(ggui.AlignCenter)
			rows = append(rows, ui.ButtonOf(row, func() { m.selectTab(tab) }).Name("Open "+sections[tab].Name).Ghost().Pad(8, 6))
		}
		return ggui.Column(rows...).Gap(4).Align(ggui.AlignStretch)
	})
}

// stepDetail says what an enabled, healthy step will do.
func (g *editor) stepDetail(tab int, h projectHealth) string {
	switch tab {
	case tabDep:
		return "Bundle the libraries the app links into the app"
	case tabSign:
		if h.Outputs[tabDMG] != "" || h.Outputs[tabPKG] != "" {
			return "Sign the app, then the installers"
		}
		return "Sign the app"
	case tabNotarize:
		if n := g.s.Project.Notarize; n != nil && n.Staple {
			return "Submit to Apple and staple the ticket"
		}
		return "Submit to Apple"
	case tabDistribution:
		return distributionSummary(g.s.Project)
	}
	if out := h.Outputs[tab]; out != "" {
		return "Writes " + g.displayPath(out)
	}
	return "Output path is resolved at build time"
}

// displayPath shortens a path inside the project directory to a relative one.
func (g *editor) displayPath(path string) string {
	if rel, err := filepath.Rel(filepath.Dir(g.s.Path), path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}

func (g *editor) actionSelect(fn func(int)) func(int) { return func(i int) { fn(i); g.invalidate() } }
func componentSidebar(m *desktopModel, v workspaceState) ggui.Widget {
	g := m.editor
	// No row is selected while the components are edited as JSON.
	selected := ggui.Bind(func() int {
		if s := m.Workspace.Get(); !s.Raw {
			return s.Component
		}
		return -1
	}, m.selectComponent)
	rows := ui.ListBox(m.Components, func(c componentRow) int { return c.Index }, func(c ggui.Readable[componentRow]) ggui.Widget {
		return ggui.TextOf(ggui.Map(c, func(c componentRow) string { return c.Label })).NoWrap().Ellipsis()
	}).RowName(func(c componentRow) string { return "Component " + c.Label }).Name("Components").BindSelected(selected)
	content := ggui.Column(ggui.TextOf(m.Components.Map(func(c []componentRow) string { return fmt.Sprintf("Components · %d", len(c)) })), ui.Caption("Installer payloads"), ggui.Expanded(ggui.Scroll(rows).BindOffset(m.offset("components"))), ui.Button("Add component", g.action(g.guard(g.addComponent))).Outline(), ui.Button("Remove", g.action(g.guard(g.removeComponent))).Ghost().BindDisabled(ggui.Derived(func() bool { return m.Workspace.Get().Raw || len(m.Components.Get()) == 0 }))).Gap(10).Align(ggui.AlignStretch)
	return ggui.Box(ui.Card(content)).Width(210)
}
func helpView(tab int) ggui.Widget {
	t := uitheme.Use()
	children := []ggui.Widget{ggui.Column(ui.Title("About this step"), ui.Caption(sections[tab].Description)).Gap(6).Align(ggui.AlignStretch)}
	for _, h := range stepHelp[tab] {
		children = append(children, ggui.Column(ggui.Text(h[0]).Size(12).Color(t.Primary), ggui.Text(h[1]).Size(13)).Gap(6).Align(ggui.AlignStretch))
	}
	return ggui.Box(ui.Card(ggui.Scroll(ggui.Padding(ggui.Column(children...).Gap(20).Align(ggui.AlignStretch), 0, scrollGutter, 0, 0)))).Width(280)
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
func removeLibrary(g *editor, i int) {
	if !g.commit() {
		return
	}
	g.s.checkpoint()
	g.s.Project.Dep.Libs = slices.Delete(g.s.Project.Dep.Libs, i, i+1)
	g.clearIssue(tabDep)
	g.rebuild()
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
