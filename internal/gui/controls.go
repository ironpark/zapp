package gui

import (
	"fmt"
	"path/filepath"

	"github.com/ironpark/zapp"
)

// guard runs action only once the field being edited commits, so an invalid
// draft is never left behind by navigating away from it.
func (g *editor) guard(action func()) func() {
	return func() {
		if g.commit() {
			action()
		}
	}
}

func (g *editor) switchPackageForm() {
	p := g.s.Project.PKG
	if !p.HasFullForm() {
		if p.HasShortForm() {
			g.report(fmt.Errorf("clear short-form fields before switching to components"), "")
			return
		}
		g.s.checkpoint()
		p.Components = []zapp.Component{g.defaultComponent()}
	} else {
		defaultComponent := len(p.Components) == 1 && p.Components[0] == g.defaultComponent()
		if (len(p.Components) > 0 && !defaultComponent) || p.Distribution != nil {
			g.report(fmt.Errorf("clear components and distribution before switching to short form"), "")
			return
		}
		g.s.checkpoint()
		p.Components = nil
	}
	g.rebuild()
}

func (g *editor) defaultComponent() zapp.Component {
	c := zapp.Component{ID: "app", InstallLocation: "/Applications"}
	if g.s.Project.App != "" {
		c.Root = filepath.Dir(g.s.Project.App)
		c.Entry = filepath.Base(g.s.Project.App)
	}
	return c
}
