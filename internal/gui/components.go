package gui

import (
	"fmt"
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/ironpark/zapp"
	"github.com/ironpark/zapp/internal/gui/comp"
)

func (g *editor) componentPanel() comp.Panel {
	return comp.Panel{Bounds: comp.Box(24, workspaceTop, 200, g.contentBottom()-workspaceTop), Title: fmt.Sprintf("Components · %d", len(g.s.Project.PKG.Components))}
}

// componentPageSize is the number of component rows that fit above the Add and
// Remove buttons. Drawing, scrolling and revealing must agree on it.
func (g *editor) componentPageSize() int {
	return max(1, (g.componentPanel().Content().Dy()-88)/40)
}
func (g *editor) componentButtons() []comp.Button {
	panel := g.componentPanel()
	area := panel.Content()
	visible := g.componentPageSize()
	start := max(0, min(g.componentScroll, len(g.s.Project.PKG.Components)-visible))
	var out []comp.Button
	for i := start; i < min(len(g.s.Project.PKG.Components), start+visible); i++ {
		label := g.s.Project.PKG.Components[i].ID
		if label == "" {
			label = fmt.Sprintf("Component %d", i+1)
		}
		out = append(out, comp.Button{Bounds: comp.Box(area.Min.X, area.Min.Y+(i-start)*40, area.Dx(), 32), Label: label, Selected: i == g.componentIndex && !g.pkgRaw, Ghost: true, OnClick: g.guard(func() {
			g.componentIndex = i
			g.pkgRaw = false
			g.pkgViewScroll[0] = 0
			g.form.ScrollTo(0)
			g.rebuild()
		})})
	}
	out = append(out,
		comp.Button{Bounds: comp.Box(area.Min.X, area.Max.Y-76, area.Dx(), 32), Label: "Add component", Icon: comp.IconPlus, OnClick: g.guard(g.addComponent)},
		comp.Button{Bounds: comp.Box(area.Min.X, area.Max.Y-36, area.Dx(), 32), Label: "Remove", Icon: comp.IconTrash, Disabled: g.pkgRaw || len(g.s.Project.PKG.Components) == 0, OnClick: g.guard(g.removeComponent)})
	return out
}
func (g *editor) addComponent() {
	c := g.s.Project.PKG
	if c.Type == "component" && len(c.Components) > 0 {
		g.showFieldError(tabPKG, "Package type", fmt.Errorf("choose product to include multiple components"))
		return
	}
	id := ""
	for n := 1; id == ""; n++ {
		candidate := fmt.Sprintf("component-%d", n)
		exists := false
		for _, item := range c.Components {
			if item.ID == candidate {
				exists = true
				break
			}
		}
		if !exists {
			id = candidate
		}
	}
	g.s.checkpoint()
	g.pkgRaw = false
	c.Components = append(c.Components, zapp.Component{ID: id, InstallLocation: "/Applications"})
	g.componentIndex = len(c.Components) - 1
	g.revealComponent()
	g.form.ScrollTo(0)
	g.rebuild()
}
func (g *editor) removeComponent() {
	c := g.s.Project.PKG
	if g.componentIndex < 0 || g.componentIndex >= len(c.Components) {
		return
	}
	id := c.Components[g.componentIndex].ID
	if c.Distribution != nil {
		for _, choice := range c.Distribution.Choices {
			for _, pkg := range choice.Packages {
				if pkg == id {
					g.report(fmt.Errorf("remove %s from distribution choices before deleting it", id), "")
					return
				}
			}
		}
	}
	g.s.checkpoint()
	c.Components = append(c.Components[:g.componentIndex], c.Components[g.componentIndex+1:]...)
	g.clearIssue(tabPKG)
	if c.Components == nil {
		c.Components = []zapp.Component{}
	}
	g.componentIndex = max(0, g.componentIndex-1)
	g.revealComponent()
	g.rebuild()
}
func (g *editor) drawComponentList(dst *ebiten.Image) {
	if !g.componentListVisible() {
		return
	}
	g.componentPanel().Draw(dst, g.ui)
	if len(g.s.Project.PKG.Components) == 0 {
		area := g.componentPanel().Content()
		g.ui.Wrapped(dst, "Add a component to configure its payload and destination.", area.Min.X, area.Min.Y, area.Dx(), 13, g.ui.Theme.Muted, 4)
	}
}
func (g *editor) scrollComponents(point image.Point, delta int) bool {
	if !g.componentListVisible() || !point.In(g.componentPanel().Bounds) || delta == 0 {
		return false
	}
	visible := g.componentPageSize()
	g.componentScroll = max(0, min(g.componentScroll+delta, len(g.s.Project.PKG.Components)-visible))
	return true
}

func (g *editor) revealComponent() {
	visible := g.componentPageSize()
	if g.componentIndex < g.componentScroll {
		g.componentScroll = g.componentIndex
	}
	if g.componentIndex >= g.componentScroll+visible {
		g.componentScroll = g.componentIndex - visible + 1
	}
}
