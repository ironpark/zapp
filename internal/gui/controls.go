package gui

import (
	"fmt"

	"github.com/ironpark/zapp"
	"github.com/ironpark/zapp/internal/gui/comp"
)

// These declarations bind reusable controls to editor actions. Guards live
// here, so comp does not need to know how a project commits or validates edits.
func (g *editor) guard(action func()) func() {
	return func() {
		if g.commit() {
			action()
		}
	}
}
func (g *editor) tabs() comp.Tabs {
	items := make([]comp.Tab, len(sections))
	for i, s := range sections {
		items[i] = comp.Tab{Label: s.Name}
	}
	var measure func(string, int) int
	if g.ui != nil {
		measure = g.ui.Measure
	}
	return comp.Tabs{Measure: measure, Bounds: comp.Box(24, 68, g.w-48-402, 44), Items: items, Selected: g.tab, Gap: 4, OnSelect: g.switchTab}
}
func (g *editor) stepToggle() comp.Toggle {
	label := "Disabled"
	if g.enabled() {
		label = "Enabled"
	}
	return comp.Toggle{Bounds: comp.Box(g.w-148, 74, 124, 32), Label: label, Checked: g.enabled(), OnChange: g.guard(g.toggle)}
}
func (g *editor) controls() []comp.Button {
	buttons := []comp.Button{
		{Bounds: comp.Box(g.w-456, 12, 36, 36), Ghost: true, Label: "Undo", Icon: comp.IconUndo, IconOnly: true, Disabled: !g.s.CanUndo(), OnClick: g.guard(func() { g.history(false) })},
		{Bounds: comp.Box(g.w-412, 12, 36, 36), Ghost: true, Label: "Redo", Icon: comp.IconRedo, IconOnly: true, Disabled: !g.s.CanRedo(), OnClick: g.guard(func() { g.history(true) })},
		{Bounds: comp.Box(g.w-240, 12, 104, 36), Ghost: true, Label: "Save", Icon: comp.IconSave, OnClick: func() { g.save() }},
		{Bounds: comp.Box(g.w-368, 12, 120, 36), Ghost: true, Label: "Validate", Icon: comp.IconCheck, OnClick: g.validate},
		{Bounds: comp.Box(g.w-128, 12, 104, 36), Ghost: true, Label: "Build", Icon: comp.IconPlay, Primary: true, Disabled: g.build != nil, OnClick: g.startBuild},
	}
	if g.tab == tabDMG && g.enabled() {
		items := g.itemsPanel().Bounds
		inspector := g.inspectorPanel().Bounds
		buttons = append(buttons,
			comp.Button{Bounds: comp.Box(items.Max.X-48, items.Min.Y+10, 32, 28), Label: "Add file", Icon: comp.IconPlus, IconOnly: true, OnClick: g.guard(g.addFile)},
			comp.Button{Bounds: comp.Box(inspector.Min.X+16, inspector.Max.Y-44, inspector.Dx()-32, 28), Label: "Remove from DMG", Icon: comp.IconTrash, Disabled: g.selected == "", OnClick: g.removeSelected},
			comp.Button{Bounds: comp.Box(g.w-302, 74, 146, 32), Label: "Default layout", Disabled: g.s.Project.DMG.Contents == nil, OnClick: g.guard(func() {
				g.s.checkpoint()
				g.s.Project.DMG.Contents = nil
				g.selected = ""
				g.rebuild()
				g.report(nil, "Default layout restored. Undo restores your custom contents.")
			})},
		)
	}
	if g.tab == tabDMG && g.enabled() && g.selected != "" {
		item, ok := g.selectedContent()
		if ok && item.Icon != "" && len(g.inspector.Inputs) > 3 {
			field := g.inspector.FieldBounds(3)
			bounds := comp.Box(g.inspector.Bounds.Max.X-26, field.Min.Y-25, 24, 22)
			if bounds.In(g.inspector.Bounds) {
				buttons = append(buttons, comp.Button{Bounds: bounds, Label: "Reset item icon", Icon: comp.IconUndo, IconOnly: true, OnClick: g.guard(func() {
					g.s.checkpoint()
					g.s.materialize()
					item := g.s.Project.DMG.Contents[g.selected]
					item.Icon = ""
					g.s.Project.DMG.Contents[g.selected] = item
					g.rebuild()
				})})
			}
		}
	}
	if g.tab == tabDMG && g.enabled() && !g.dmgYAML {
		panel := g.settingsPanel().Bounds
		icon := comp.IconChevronDown
		if g.dmgAdvanced {
			icon = comp.IconChevronUp
		}
		buttons = append(buttons, comp.Button{Bounds: comp.Box(panel.Min.X+16, panel.Max.Y-48, panel.Dx()-32, 32), Label: "Advanced settings", Icon: icon, Selected: g.dmgAdvanced, OnClick: g.guard(func() {
			g.dmgAdvanced = !g.dmgAdvanced
			g.rebuild()
			if g.dmgAdvanced {
				g.form.ScrollBy(g.form.FieldBounds(6).Min.Y - g.form.Bounds.Min.Y - 25)
			} else {
				g.form.ScrollTo(0)
			}
		})})
	}
	if g.tab == tabPKG && g.enabled() {
		buttons = append(buttons, comp.Button{Bounds: comp.Box(g.w-376, 74, 220, 32), Label: "Switch package form", OnClick: g.guard(g.switchPackageForm)})
	}
	return buttons
}
func (g *editor) switchPackageForm() {
	p := g.s.Project.PKG
	if !p.HasFullForm() {
		if p.HasShortForm() {
			g.report(fmt.Errorf("clear short-form fields before switching to components"), "")
			return
		}
		g.s.checkpoint()
		p.Components = []zapp.Component{{ID: "app", Root: g.s.Project.App, InstallLocation: "/Applications"}}
	} else {
		if len(p.Components) > 0 || p.Distribution != nil {
			g.report(fmt.Errorf("clear components and distribution before switching to short form"), "")
			return
		}
		g.s.checkpoint()
		p.Components = nil
	}
	g.rebuild()
}
func (g *editor) closeDialog() comp.Dialog {
	return comp.Dialog{Visible: g.confirmClose, Bounds: comp.Center(comp.Box(0, 0, g.w, g.h), 500, 190), Title: "Save changes before closing?", Message: "Your project has unsaved edits.", OnCancel: func() { g.confirmClose = false }, Actions: []comp.Button{
		{Label: "Save & close", Primary: true, OnClick: func() {
			if g.save() {
				g.quit = true
			} else {
				g.confirmClose = false
			}
		}},
		{Label: "Discard changes", OnClick: func() { g.quit = true }},
		{Label: "Keep editing", OnClick: func() { g.confirmClose = false }},
	}}
}
