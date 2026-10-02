package gui

import (
	"fmt"
	"slices"

	"github.com/ironpark/zapp"
)

func (g *editor) addComponent() {
	c := g.s.Project.PKG
	if c.Type == "component" && len(c.Components) > 0 {
		g.showIssue(at(tabPKG, labelPackageType), fmt.Errorf("choose product to include multiple components"))
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
	g.rebuild()
}
