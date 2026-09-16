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
	return comp.Tabs{Measure: measure, Bounds: comp.Box(24, 84, g.w-48, 44), Items: items, Selected: g.tab, Gap: 4, OnSelect: g.switchTab}
}
func (g *editor) stepToggle() comp.Toggle {
	label := g.section().Name + " disabled"
	if g.enabled() {
		label = g.section().Name + " enabled"
	}
	return comp.Toggle{Bounds: comp.Box(32, 145, 250, 34), Label: label, Checked: g.enabled(), OnChange: g.guard(g.toggle)}
}
func (g *editor) controls() []comp.Button {
	buttons := []comp.Button{
		{Bounds: comp.Box(g.w-338, 24, 84, 36), Label: "Undo", Disabled: !g.s.CanUndo(), OnClick: g.guard(func() { g.s.Undo(); g.rebuild() })},
		{Bounds: comp.Box(g.w-246, 24, 84, 36), Label: "Redo", Disabled: !g.s.CanRedo(), OnClick: g.guard(func() { g.s.Redo(); g.rebuild() })},
		{Bounds: comp.Box(g.w-154, 24, 130, 36), Label: "Save project", Primary: true, OnClick: func() { g.save() }},
		{Bounds: comp.Box(g.w-130, g.h-footerHeight+6, 106, 28), Label: "Validate", OnClick: g.validate},
	}
	if g.tab == tabDMG && g.enabled() {
		buttons = append(buttons,
			comp.Button{Bounds: comp.Box(g.w-440, 205, 68, 28), Label: "Fit", Selected: !g.previewActual, OnClick: func() { g.previewActual = false; g.panX = 0; g.panY = 0 }},
			comp.Button{Bounds: comp.Box(g.w-364, 205, 64, 28), Label: "100%", Selected: g.previewActual, OnClick: func() { g.previewActual = true; g.panX = 0; g.panY = 0 }},
			comp.Button{Bounds: comp.Box(404, 145, 104, 34), Label: "Add file", OnClick: g.guard(g.addFile)},
			comp.Button{Bounds: comp.Box(518, 145, 142, 34), Label: "Remove selected", Disabled: g.selected == "", OnClick: g.guard(func() {
				g.s.checkpoint()
				g.s.materialize()
				delete(g.s.Project.DMG.Contents, g.selected)
				g.selected = ""
				g.rebuild()
			})},
			comp.Button{Bounds: comp.Box(670, 145, 146, 34), Label: "Default layout", Disabled: g.s.Project.DMG.Contents == nil, OnClick: g.guard(func() { g.s.checkpoint(); g.s.Project.DMG.Contents = nil; g.selected = ""; g.rebuild() })},
			comp.Button{Bounds: comp.Box(826, 145, 120, 34), Label: "Advanced", Selected: g.dmgAdvanced, OnClick: g.guard(func() { g.dmgAdvanced = !g.dmgAdvanced; g.form.ScrollTo(0); g.rebuild() })},
		)
	}
	if g.tab == tabPKG && g.enabled() {
		buttons = append(buttons, comp.Button{Bounds: comp.Box(294, 145, 220, 34), Label: "Switch package form", OnClick: g.guard(g.switchPackageForm)})
	}
	return buttons
}
func (g *editor) switchPackageForm() {
	p := g.s.Project.PKG
	if p.Components == nil && p.Distribution == nil {
		if p.Identifier != "" || p.Version != "" || p.InstallLocation != "" || p.Scripts != "" || p.MinOS != "" || p.License != nil {
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
