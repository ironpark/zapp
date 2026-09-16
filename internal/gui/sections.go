package gui

import "github.com/ironpark/zapp"

// Tab indices. They index sections, so the two stay in step by construction.
const (
	tabProject = iota
	tabDMG
	tabPKG
	tabDep
	tabSign
	tabNotarize
)

// section describes one editor tab: how it is labelled and described, how its
// fields are built, and — for the optional project steps — how it is switched
// on and off. A nil presence means the tab is always available.
type section struct {
	Name        string
	Description string
	// Credentials marks steps whose help text warns about secret handling.
	Credentials bool
	fields      func(*editor) []field
	present     func(*zapp.Project) bool
	flip        func(p, scratch *zapp.Project)
}

// Optional reports whether the section can be enabled and disabled.
func (s section) Optional() bool { return s.present != nil }

// Enabled reports whether the section contributes to the project as it stands.
func (s section) Enabled(p *zapp.Project) bool { return s.present == nil || s.present(p) }

// toggleSection moves a section between the project and the scratch project
// that remembers disabled values, so re-enabling restores what was filled in.
func toggleSection[T any](live, stash **T) {
	if *live != nil {
		*stash, *live = *live, nil
		return
	}
	if *stash == nil {
		*stash = new(T)
	}
	*live = *stash
}

var sections = []section{{
	Name:        "Project",
	Description: "Choose the app bundle and output directory shared by your packaging steps.",
	fields:      (*editor).projectFields,
}, {
	Name:        "DMG",
	Description: "Arrange the installer window and its contents.",
	fields:      (*editor).dmgFields,
	present:     func(p *zapp.Project) bool { return p.DMG != nil },
	flip:        func(p, s *zapp.Project) { toggleSection(&p.DMG, &s.DMG) },
}, {
	Name:        "PKG",
	Description: "Configure the installer identity, destination and package contents.",
	fields:      (*editor).pkgFields,
	present:     func(p *zapp.Project) bool { return p.PKG != nil },
	flip:        func(p, s *zapp.Project) { toggleSection(&p.PKG, &s.PKG) },
}, {
	Name:        "Dependencies",
	Description: "Bundle the libraries your app needs and configure dependency search paths.",
	fields:      (*editor).depFields,
	present:     func(p *zapp.Project) bool { return p.Dep != nil },
	flip:        func(p, s *zapp.Project) { toggleSection(&p.Dep, &s.Dep) },
}, {
	Name:        "Signing",
	Description: "Choose credentials to sign your app and installers.",
	Credentials: true,
	fields:      (*editor).signFields,
	present:     func(p *zapp.Project) bool { return p.Sign != nil },
	flip:        func(p, s *zapp.Project) { toggleSection(&p.Sign, &s.Sign) },
}, {
	Name:        "Notarization",
	Description: "Configure Apple notarization and ticket stapling for distribution.",
	Credentials: true,
	fields:      (*editor).notarizeFields,
	present:     func(p *zapp.Project) bool { return p.Notarize != nil },
	flip:        func(p, s *zapp.Project) { toggleSection(&p.Notarize, &s.Notarize) },
}}

// noSections reports whether the project enables no optional step at all.
func noSections(p *zapp.Project) bool {
	for _, s := range sections {
		if s.Optional() && s.present(p) {
			return false
		}
	}
	return true
}

func (g *editor) section() section { return sections[g.tab] }
func (g *editor) enabled() bool    { return g.section().Enabled(g.s.Project) }

func (g *editor) toggle() {
	g.clearIssue(g.tab)
	g.s.checkpoint()
	// Disabled values stay in g.disabled so toggling a section back on restores
	// its fields. Only enabled sections are persisted.
	g.section().flip(g.s.Project, &g.disabled)
	g.selected = ""
	g.rebuild()
}
