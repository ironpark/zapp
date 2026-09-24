package gui

import (
	"fmt"
	"slices"

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
func (g *editor) addComponent() {
	c := g.s.Project.PKG
	if c.Type == "component" && len(c.Components) > 0 {
		g.showFieldError(tabPKG, "Package type", fmt.Errorf("choose product to include multiple components"))
		return
	}
	id := ""
	for n := 1; id == ""; n++ {
		candidate := fmt.Sprintf("component-%d", n)
		if !slices.ContainsFunc(c.Components, func(item zapp.Component) bool { return item.ID == candidate }) {
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
			if slices.Contains(choice.Packages, id) {
				g.report(fmt.Errorf("remove %s from distribution choices before deleting it", id), "")
				return
			}
		}
	}
	g.s.checkpoint()
	c.Components = slices.Delete(c.Components, g.componentIndex, g.componentIndex+1)
	g.clearIssue(tabPKG)
	if c.Components == nil {
		c.Components = []zapp.Component{}
	}
	g.componentIndex = max(0, g.componentIndex-1)
	g.revealComponent()
	g.rebuild()
}

func (g *editor) revealComponent() {
	if g.desktop != nil {
		g.desktop.revealComponent = g.componentIndex
		return
	}
	visible := g.componentPageSize()
	if g.componentIndex < g.componentScroll {
		g.componentScroll = g.componentIndex
	}
	if g.componentIndex >= g.componentScroll+visible {
		g.componentScroll = g.componentIndex - visible + 1
	}
}
