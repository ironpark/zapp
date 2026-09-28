package gui

import (
	"fmt"
	"image/color"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/ironpark/ggui"
	"github.com/ironpark/ggui/ui"
	"github.com/ironpark/ggui/ui/icons"
	"github.com/ironpark/ggui/ui/icons/lucide"
	uitheme "github.com/ironpark/ggui/ui/theme"
	"github.com/ironpark/zapp/internal/gui/comp"
)

func desktopView(m *desktopModel) ggui.Widget {
	return ggui.Reactive(func() ggui.Widget {
		t := uitheme.Use()
		g := m.editor
		toolbar := ggui.Padding(ggui.Row(
			ggui.Box(ggui.Text("Z").Color(t.PrimaryFg)).Fill(t.Primary).Radius(4).Pad(5, 9),
			ggui.Expanded(ggui.Column(ggui.TextOf(m.Title).Size(15).NoWrap(), ui.Caption(filepath.Join(g.s.Dir, g.s.Name)).NoWrap()).Gap(3)),
			ggui.TextOf(m.SaveState).Size(12).Color(t.MutedFg),
			ui.Tooltip(ghostIcon(icons.New(undoIcon()), "Undo", g.action(g.guard(func() { g.history(false) }))).BindDisabled(m.CanUndo.Map(func(v bool) bool { return !v })), "Undo · "+shortcut("Z")),
			ui.Tooltip(ghostIcon(icons.New(redoIcon()), "Redo", g.action(g.guard(func() { g.history(true) }))).BindDisabled(m.CanRedo.Map(func(v bool) bool { return !v })), "Redo · "+shortcut("Shift+Z")),
			healthBadge(m),
			ui.Tooltip(ui.Button("Save", g.action(func() { g.save() })).Outline().BindDisabled(m.Busy), "Save · "+shortcut("S")),
			ui.Tooltip(ui.Button("Build", g.action(g.startBuild)).BindDisabled(m.Busy), "Build · "+shortcut("B")),
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
			return ggui.Padding(ggui.Row(ggui.Text("●").Color(color), ggui.Expanded(ui.Tooltip(ggui.Text(v.message).Size(12).Color(color).NoWrap(), v.message)), ggui.If(m.Workspace.Map(func(v workspaceState) bool { return v.IssueTab >= 0 }), func() ggui.Widget { return ui.Button("Go to issue", g.action(g.goToIssue)).Ghost() })).Gap(8), 5, 16)
		})
		page := ggui.View(m.Workspace, func(v workspaceState) ggui.Widget { return workspaceView(m, v) })
		body := ggui.Column(toolbar, ui.Divider(), stepTabs(m), ui.Divider(), ggui.Expanded(page), ui.Divider(), footer).Align(ggui.AlignStretch)
		return ggui.Column(ggui.Expanded(body), closeDialogView(m), ggui.View(m.Modal, func(v modalState) ggui.Widget { return buildDialogView(m, v) })).Align(ggui.AlignStretch)
	})
}

// shortcut spells a command chord the way the platform's menus do.
func shortcut(keys string) string {
	if runtime.GOOS == "darwin" {
		return strings.ReplaceAll("⌘"+keys, "Shift+", "⇧")
	}
	return "Ctrl+" + keys
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
		label, tip := "Ready", "Build inputs look complete · Validate "+shortcut("Shift+V")
		if count > 0 {
			label, tip = fmt.Sprintf("%d issue%s", count, plural(count)), "Show the first issue · "+shortcut("Shift+V")
		}
		content := ggui.Row(lucide.Icon(icon).Size(14).Color(col), ggui.Text(label).Color(col)).Gap(6).Align(ggui.AlignCenter)
		return ui.Tooltip(ui.ButtonOf(content, g.action(g.validate)).Name("Validate").Ghost().BindDisabled(m.Busy), tip)
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

// stepTabs is the tab strip. Unlike a plain tab bar it shows each step's
// state: a disabled step is dimmed and a step with a problem carries a dot,
// whose tooltip names the problem.
func stepTabs(m *desktopModel) ggui.Widget {
	return ggui.Reactive(func() ggui.Widget {
		t := uitheme.Use()
		current, health, enabled := m.Tab.Get(), m.Health.Get(), m.Enabled.Get()
		tabs := []ggui.Widget{}
		for i, s := range sections {
			col := t.MutedFg
			if i == current {
				col = t.Fg
			}
			if !enabled[i] {
				col = fade(t.MutedFg, .55)
			}
			label := []ggui.Widget{ggui.Text(s.Name).Color(col).NoWrap()}
			if health.Issues[i] != "" {
				label = append(label, ggui.Box().Size(6, 6).Radius(3).Fill(t.Destructive))
			}
			var tab ggui.Widget = ui.ButtonOf(ggui.Row(label...).Gap(6).Align(ggui.AlignCenter), func() { m.selectTab(i) }).Name(s.Name).Ghost().Pad(7, 10)
			if i == current {
				tab = underline(tab, t.Primary)
			}
			switch {
			case health.Issues[i] != "":
				tab = ui.Tooltip(tab, health.Issues[i])
			case !enabled[i]:
				tab = ui.Tooltip(tab, s.Name+" is off")
			}
			tabs = append(tabs, tab)
		}
		return ggui.Padding(ggui.Row(tabs...).Gap(2), 0, 6)
	})
}

// underline draws a 2 px bar of col beneath child, the width of child.
func underline(child ggui.Widget, col color.Color) ggui.Widget {
	const thickness = 2
	return ggui.FromFuncs(func(c ggui.Constraints, env ggui.Env) ggui.Size {
		size := child.Layout(c, env)
		return ggui.Sz(size.W, size.H+thickness)
	}, func(dst *ggui.Canvas, r ggui.Rect) {
		dst.Paint(child, ggui.Rct(r.Origin, ggui.Sz(r.Size.W, r.Size.H-thickness)))
		dst.FillRoundRect(ggui.Rct(ggui.Pt(r.Origin.X, r.Origin.Y+r.Size.H-thickness), ggui.Sz(r.Size.W, thickness)), 0, col)
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
	g := m.editor
	if v.Tab == tabSign {
		children = append(children, ggui.Row(modeButtons([]string{"Keychain", "PKCS#12", "PEM"}, v.SignMethod, g.actionSelect(g.selectSignMethod)),
			ui.Tooltip(ui.Button("Check", g.action(g.checkSigning)).Outline().Disabled(v.Check.Checking), "Find the certificate these settings sign with, without building")).Gap(16).Align(ggui.AlignCenter))
		if v.Check.Shown {
			children = append(children, checkResult(v))
		}
	}
	if v.Tab == tabNotarize {
		children = append(children, modeButtons([]string{"Profile", "Apple ID", "API key"}, v.NotaryMethod, g.actionSelect(g.selectNotaryMethod)))
	}
	var content ggui.Widget = formView(m, m.Fields)
	switch {
	case v.Tab == tabProject:
		content = ggui.Column(appCard(m), content, buildSteps(m)).Gap(28).Align(ggui.AlignStretch)
	case v.Tab == tabSign && v.SignMethod == 0 && v.IdentitiesListed:
		content = ggui.Column(content, identitySuggestions(m, v)).Gap(20).Align(ggui.AlignStretch)
	}
	if v.Tab != tabDMG && !v.Raw {
		content = maxWidth(content, formMaxWidth)
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
	return surface(ggui.Column(children...).Gap(12).Align(ggui.AlignStretch))
}

// formMaxWidth keeps single forms readable in a wide window: labels, inputs
// and their Browse buttons stay within one glance.
const formMaxWidth = 760

// maxWidth lays child out no wider than w, aligned to the start.
func maxWidth(child ggui.Widget, w float64) ggui.Widget {
	var width float64
	return ggui.FromFuncs(func(c ggui.Constraints, env ggui.Env) ggui.Size {
		inner := c
		inner.MaxW = min(c.MaxW, w)
		inner.MinW = min(c.MinW, inner.MaxW)
		size := child.Layout(inner, env)
		width = size.W
		return c.Constrain(size)
	}, func(dst *ggui.Canvas, r ggui.Rect) {
		dst.Paint(child, ggui.Rct(r.Origin, ggui.Sz(min(r.Size.W, width), r.Size.H)))
	})
}

// checkResult shows what the Signing tab's Check found.
func checkResult(v workspaceState) ggui.Widget {
	t := uitheme.Use()
	if v.Check.Checking {
		return ui.Caption("Checking the certificate…")
	}
	icon, col := statusIcon(t, v.Check.OK)
	return ggui.Row(lucide.Icon(icon).Size(14).Color(col), ggui.Expanded(ggui.Text(v.Check.Message).Size(12).Color(col))).Gap(6).Align(ggui.AlignStart)
}

// identitySuggestions offers the keychain's signing identities to fill the
// Keychain method's identity with one click.
func identitySuggestions(m *desktopModel, v workspaceState) ggui.Widget {
	g := m.editor
	refresh := ui.Tooltip(ui.Button("Refresh", g.action(g.refreshIdentities)).Ghost().Pad(2, 8).Disabled(v.IdentitiesListing),
		"Look up the keychain again, as after importing a certificate")
	children := []ggui.Widget{ggui.Row(ggui.Expanded(ggui.Text("In your keychain").Size(12).Color(uitheme.Use().Primary)), refresh).Align(ggui.AlignCenter)}
	if v.Identities == "" {
		children = append(children, ui.Caption("No valid signing identities were found. Import your Developer ID certificate, then refresh, or use the PKCS#12 or PEM method."))
	} else {
		for _, name := range strings.Split(v.Identities, "\n") {
			children = append(children, ui.ButtonOf(ggui.Row(ggui.Expanded(ggui.Text(name).NoWrap())), g.action(func() { g.useIdentity(name) })).Name("Use "+name).Outline())
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
		return ggui.Row(icon, ggui.Expanded(ggui.Column(ggui.Text(title).Size(16).NoWrap(), caption).Gap(4))).Gap(14).Align(ggui.AlignCenter)
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
			row := ggui.Row(lucide.Icon(icon).Size(16).Color(col), ggui.Expanded(ggui.Column(ggui.Text(sections[tab].Name), ggui.Text(detail).Size(12).Color(detailCol).NoWrap()).Gap(3)), lucide.Icon("chevron-right").Size(14).Color(t.MutedFg)).Gap(12).Align(ggui.AlignCenter)
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
	rows := ggui.EachKeyed(m.Components, func(c componentRow) int { return c.Index }, func(row ggui.EachItem[componentRow]) ggui.Widget {
		return ggui.Reactive(func() ggui.Widget {
			c := row.Value.Get()
			state := m.Workspace.Get()
			b := ui.ButtonOf(ggui.Row(ggui.Expanded(ggui.Text(c.Label).NoWrap())), func() { m.selectComponent(c.Index) }).Name("Component "+c.Label).Pad(8, 10)
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
	}).Gap(4).Align(ggui.AlignStretch)
	content := ggui.Column(ggui.TextOf(m.Components.Map(func(c []componentRow) string { return fmt.Sprintf("Components · %d", len(c)) })), ui.Caption("Installer payloads"), ggui.Expanded(ggui.Scroll(rows)), ui.Button("Add component", g.action(g.guard(g.addComponent))).Outline(), ui.Button("Remove", g.action(g.guard(g.removeComponent))).Ghost().BindDisabled(ggui.Derived(func() bool { return m.Workspace.Get().Raw || len(m.Components.Get()) == 0 }))).Gap(10).Align(ggui.AlignStretch)
	return ggui.Box(surface(content)).Width(210)
}
func helpView(tab int) ggui.Widget {
	t := uitheme.Use()
	children := []ggui.Widget{ggui.Column(ui.Title("About this step"), ui.Caption(sections[tab].Description)).Gap(6).Align(ggui.AlignStretch)}
	for _, h := range stepHelp[tab] {
		children = append(children, ggui.Column(ggui.Text(h[0]).Size(12).Color(t.Primary), ggui.Text(h[1]).Size(13)).Gap(6).Align(ggui.AlignStretch))
	}
	return ggui.Box(surface(ggui.Scroll(ggui.Padding(ggui.Column(children...).Gap(20).Align(ggui.AlignStretch), 0, scrollGutter, 0, 0)))).Width(280)
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
func buildDialogView(m *desktopModel, v modalState) ggui.Widget {
	if v.Title == "" {
		return ggui.Box()
	}
	g := m.editor
	actions := []ggui.Widget{}
	body := []ggui.Widget{ggui.Text(v.Message)}
	if v.Build {
		if v.Log != "" {
			t := uitheme.Use()
			log := ggui.Scroll(ggui.Padding(ggui.Text(v.Log).Style(ggui.TextStyle{Font: codeFont(), Size: 12}).Color(t.MutedFg), 10, 12)).BindOffset(m.offset("build-log"))
			body = append(body, ggui.Box(log).Height(220).Fill(t.Muted).Radius(t.Radius))
			actions = append(actions, ui.Button("Copy log", func() { ggui.CurrentClipboard().Write(v.Log) }).Ghost(), ggui.Spacer())
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

var (
	undoIcon = sync.OnceValue(func() *icons.SVG { return compIcon(comp.IconUndo) })
	redoIcon = sync.OnceValue(func() *icons.SVG { return compIcon(comp.IconRedo) })
)

func compIcon(name comp.Icon) *icons.SVG {
	icon, err := icons.Load(comp.IconFiles(), name.File())
	if err != nil {
		panic(err)
	}
	return icon
}

// Keep icon creation close to its named control so accessibility has a textual
// action even when the visual label is only an icon.
func iconButton(icon, name string, action func()) ggui.Widget {
	return ui.Tooltip(ghostIcon(lucide.Icon(icon), name, action), name)
}

// ghostIcon is the borderless button behind every icon-only control.
func ghostIcon(icon *icons.Widget, name string, action func()) *ui.ButtonWidget {
	return ui.ButtonOf(icon.Size(16), action).Name(name).Ghost().Pad(6)
}
