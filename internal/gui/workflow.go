package gui

import (
	"fmt"
	"image"
	"runtime"

	"github.com/ironpark/zapp/internal/gui/comp"
)

type validationIssue struct {
	tab            int
	label, message string
	component      int
	item           string
}

// issueOn reports whether the outstanding validation issue belongs to tab.
func (g *editor) issueOn(tab int) bool { return g.issue != nil && g.issue.tab == tab }

// clearIssue drops the outstanding validation issue when it belongs to tab, so
// every mutation path states the rule the same way.
func (g *editor) clearIssue(tab int) {
	if g.issueOn(tab) {
		g.issue = nil
	}
}

// switchMethod moves credential values between a live config and its stash when
// the user picks a different method. groups[i] lists the (live, stash) pointer
// pairs owned by method i; fields outside every group are left untouched.
func switchMethod(current, next int, groups [][][2]*string) {
	for _, pair := range groups[current] {
		*pair[1] = *pair[0]
	}
	for _, group := range groups {
		for _, pair := range group {
			*pair[0] = ""
		}
	}
	for _, pair := range groups[next] {
		*pair[0] = *pair[1]
	}
}

func (g *editor) signMethod() int {
	c := g.s.Project.Sign
	if c != nil {
		if c.P12File != "" || c.P12PasswordFile != "" {
			return 1
		}
		if c.PEMFile != "" {
			return 2
		}
		if c.Identity != "" {
			return 0
		}
	}
	if g.signModeSet {
		return g.signMode
	}
	if runtime.GOOS != "darwin" {
		return 1
	}
	return 0
}
func (g *editor) notaryMethod() int {
	c := g.s.Project.Notarize
	if c != nil {
		if c.APIKeyFile != "" {
			return 2
		}
		if c.Profile != "" {
			return 0
		}
		if c.AppleID != "" || c.TeamID != "" || c.Password != "" {
			return 1
		}
	}
	if g.notaryModeSet {
		return g.notaryMode
	}
	if runtime.GOOS != "darwin" {
		return 2
	}
	return 0
}
func (g *editor) selectSignMethod(index int) {
	if !g.commit() || index == g.signMethod() {
		return
	}
	c, stash := g.s.Project.Sign, &g.signStash
	g.s.checkpoint()
	switchMethod(g.signMethod(), index, [][][2]*string{
		{{&c.Identity, &stash.Identity}},
		{{&c.P12File, &stash.P12File}, {&c.P12PasswordFile, &stash.P12PasswordFile}, {&c.P12Password, &stash.P12Password}},
		{{&c.PEMFile, &stash.PEMFile}},
	})
	g.issue = nil
	g.signMode, g.signModeSet = index, true
	g.rebuild()
}
func (g *editor) selectNotaryMethod(index int) {
	if !g.commit() || index == g.notaryMethod() {
		return
	}
	c, stash := g.s.Project.Notarize, &g.notaryStash
	g.s.checkpoint()
	// Staple is not owned by any method, so switchMethod leaves it in place.
	switchMethod(g.notaryMethod(), index, [][][2]*string{
		{{&c.Profile, &stash.Profile}},
		{{&c.AppleID, &stash.AppleID}, {&c.TeamID, &stash.TeamID}, {&c.Password, &stash.Password}},
		{{&c.APIKeyFile, &stash.APIKeyFile}},
	})
	g.issue = nil
	g.notaryMode, g.notaryModeSet = index, true
	g.rebuild()
}
func (g *editor) componentListVisible() bool {
	return g.tab == tabPKG && g.s.Project.PKG != nil && g.s.Project.PKG.HasFullForm()
}
func (g *editor) workflowSegments() []comp.Segmented {
	if g.tab == tabDMG || !g.enabled() {
		return nil
	}
	panel := g.settingsPanel().Bounds
	var out []comp.Segmented
	switch g.tab {
	case tabSign:
		out = append(out, comp.Segmented{Bounds: comp.Box(panel.Min.X+16, panel.Min.Y+48, panel.Dx()-32, 32), Labels: []string{"Keychain", "PKCS#12", "PEM"}, Selected: g.signMethod(), OnSelect: g.selectSignMethod})
	case tabNotarize:
		out = append(out, comp.Segmented{Bounds: comp.Box(panel.Min.X+16, panel.Min.Y+48, panel.Dx()-32, 32), Labels: []string{"Profile", "Apple ID", "API key"}, Selected: g.notaryMethod(), OnSelect: g.selectNotaryMethod})
	case tabPKG:
		mode := 0
		if g.s.Project.PKG.HasFullForm() {
			mode = 1
		}
		out = append(out, comp.Segmented{Bounds: comp.Box(g.w-384, 74, 228, 32), Labels: []string{"Single app", "Components"}, Selected: mode, OnSelect: func(_ int) {
			if g.commit() {
				g.switchPackageForm()
			}
		}})
		if mode == 1 {
			out = append(out, g.packageSourceSegment(panel))
		}
	case tabDep:
		out = append(out, g.sourceSegment(panel, g.depRaw, func(raw bool) { g.depRaw = raw }))
	}
	return out
}
func (g *editor) sourceSegment(panel image.Rectangle, raw bool, set func(bool)) comp.Segmented {
	mode := 0
	if raw {
		mode = 1
	}
	labels := []string{"Form", "JSON"}
	if g.tab == tabDep {
		labels = []string{"List", "Text"}
	}
	return comp.Segmented{Bounds: comp.Box(panel.Max.X-220, panel.Min.Y+8, 128, 32), Labels: labels, Selected: mode, OnSelect: func(index int) {
		if g.commit() {
			set(index == 1)
			g.form.ScrollTo(0)
			g.rebuild()
		}
	}}
}
func (g *editor) workflowButtons() []comp.Button {
	var out []comp.Button
	if g.tab != tabDMG && g.enabled() {
		panel := g.settingsPanel().Bounds
		out = append(out, comp.Button{Bounds: comp.Box(panel.Max.X-80, panel.Min.Y+8, 64, 32), Label: "Help", Ghost: true, Selected: g.helpOpen, OnClick: func() { g.helpOpen = !g.helpOpen; g.syncForm() }})
		if g.tab == tabPKG {
			icon := comp.IconChevronDown
			if g.pkgAdvanced {
				icon = comp.IconChevronUp
			}
			out = append(out, comp.Button{Bounds: comp.Box(panel.Min.X+16, panel.Max.Y-48, panel.Dx()-32, 32), Label: "Advanced settings", Icon: icon, Selected: g.pkgAdvanced, OnClick: g.guard(func() {
				g.pkgAdvanced = !g.pkgAdvanced
				g.rebuild()
				if g.pkgAdvanced {
					for i, f := range g.fields {
						if f.Label == "Scripts directory" || f.Label == "Distribution" {
							g.form.ScrollBy(g.form.FieldBounds(i).Min.Y - g.form.Bounds.Min.Y - 25)
							break
						}
					}
				} else {
					g.form.ScrollTo(0)
				}
			})})
		}
		if g.tab == tabDep && !g.depRaw {
			out = append(out, comp.Button{Bounds: comp.Box(panel.Min.X+16, panel.Max.Y-48, panel.Dx()-32, 32), Label: "Add directory", Icon: comp.IconPlus, OnClick: g.guard(func() { g.browse(len(g.s.Project.Dep.Libs)) })})
			for i := range g.s.Project.Dep.Libs {
				r := g.form.FieldBounds(i)
				bounds := comp.Box(r.Max.X+90-28, r.Min.Y-25, 28, 24)
				if bounds.In(g.form.Bounds) {
					out = append(out, comp.Button{Bounds: bounds, Label: "Remove path", Icon: comp.IconTrash, IconOnly: true, Ghost: true, OnClick: g.guard(func() {
						g.s.checkpoint()
						c := g.s.Project.Dep
						c.Libs = append(c.Libs[:i], c.Libs[i+1:]...)
						g.clearIssue(tabDep)
						g.rebuild()
					})})
				}
			}
		}
		if g.componentListVisible() {
			out = append(out, g.componentButtons()...)
		}
	}
	if g.issue != nil {
		out = append(out, comp.Button{Bounds: goToIssueBounds(g.w, g.h), Label: "Go to issue", Ghost: true, OnClick: g.goToIssue})
	}
	return out
}

// goToIssueBounds is shared with the footer so the status text knows exactly how
// much room the button takes instead of mirroring its width.
func goToIssueBounds(w, h int) image.Rectangle {
	return comp.Box(w-132, h-footerHeight, 116, footerHeight)
}
func (g *editor) goToIssue() {
	if g.issue == nil {
		return
	}
	issue := *g.issue
	if !g.commit() {
		return
	}
	g.componentIndex = issue.component
	g.revealComponent()
	g.selected = issue.item
	g.showFieldError(issue.tab, issue.label, fmt.Errorf("%s", issue.message))
}

// Keep each editor's scroll position while preserving the surrounding layout.
func (g *editor) packageSourceSegment(panel image.Rectangle) comp.Segmented {
	mode := 0
	if g.pkgRaw {
		mode = 1
	}
	return comp.Segmented{Bounds: comp.Box(panel.Max.X-220, panel.Min.Y+8, 128, 32), Labels: []string{"Form", "JSON"}, Selected: mode, OnSelect: func(index int) {
		if index == mode || !g.commit() {
			return
		}
		g.pkgViewScroll[mode] = g.form.Offset()
		g.pkgRaw = index == 1
		g.rebuild()
		g.form.ScrollTo(g.pkgViewScroll[index])
	}}
}
