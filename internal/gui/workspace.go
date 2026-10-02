package gui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ironpark/ggui"
	"github.com/ironpark/ggui/ui"
	"github.com/ironpark/ggui/ui/icons/lucide"
	uitheme "github.com/ironpark/ggui/ui/theme"
)

// The page under the tabs: a step's header, its settings form and the
// panels beside it.

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
			g.invalidate()
		}))
	}
	if v.Tab != tabDMG && v.Enabled {
		actions = append(actions, ui.Button("Help", func() { g.helpOpen = !g.helpOpen; g.invalidate() }).Ghost())
	}
	if sections[v.Tab].Optional() {
		enabled := ggui.Controlled(v.Enabled, func(bool) { g.guard(g.toggle)(); g.invalidate() })
		actions = append(actions, ui.Switch(enabled, status).Name("Enable "+sections[v.Tab].Name))
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
		method := modeButtons("Signing method", signMethodLabels, v.SignMethod, g.actionSelect(g.selectSignMethod))
		checkButton := ui.Button("Check", g.action(g.checkSigning)).Outline().BindDisabled(check.Map(func(c signCheck) bool { return c.Checking }))
		result := ggui.If(check.Map(func(c signCheck) bool { return c.Shown }), func() ggui.Widget { return ggui.View(check, checkResult) })
		children = append(children,
			ggui.Row(method, ui.Tooltip(checkButton, "Find the certificate these settings sign with, without building")).Gap(16).Align(ggui.AlignCenter),
			result)
	}
	if v.Tab == tabNotarize {
		children = append(children, modeButtons("Notarization method", notaryMethodLabels, v.NotaryMethod, g.actionSelect(g.selectNotaryMethod)))
	}
	var content ggui.Widget = formView(m, m.Fields)
	switch {
	case v.Tab == tabProject:
		content = ggui.Column(appCard(m), content, buildSteps(m)).Gap(28).Align(ggui.AlignStretch)
	case v.Tab == tabSign && v.SignMethod == signKeychain:
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
		children = append(children, ui.Caption("No valid signing identities were found. "+
			"Import your Developer ID certificate, then refresh, or use the PKCS#12 or PEM method."))
	} else {
		for _, name := range strings.Split(v.Identities, "\n") {
			label := ggui.Row(ggui.Expanded(ggui.Text(name).NoWrap().Ellipsis()))
			children = append(children, ui.ButtonOf(label, g.action(func() { g.useIdentity(name) })).Name("Use "+name).Outline())
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
			text := ggui.Column(ggui.Text(sections[tab].Name), ggui.Text(detail).Size(12).Color(detailCol).NoWrap().Ellipsis()).Gap(3)
			row := ggui.Row(lucide.Icon(icon).Size(16).Color(col), ggui.Expanded(text), lucide.Icon("chevron-right").Size(14).Color(t.MutedFg)).
				Gap(12).Align(ggui.AlignCenter)
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
	count := m.Components.Map(func(c []componentRow) string { return fmt.Sprintf("Components · %d", len(c)) })
	cannotRemove := ggui.Derived(func() bool { return m.Workspace.Get().Raw || len(m.Components.Get()) == 0 })
	content := ggui.Column(
		ggui.TextOf(count),
		ui.Caption("Installer payloads"),
		ggui.Expanded(ggui.Scroll(rows).BindOffset(m.offset("components"))),
		ui.Button("Add component", g.action(g.guard(g.addComponent))).Outline(),
		ui.Button("Remove", g.action(g.guard(g.removeComponent))).Ghost().BindDisabled(cannotRemove),
	).Gap(10).Align(ggui.AlignStretch)
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
