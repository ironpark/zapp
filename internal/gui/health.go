package gui

import (
	"path/filepath"
	"reflect"
	"strings"

	"github.com/ironpark/zapp"
	"github.com/ironpark/zapp/pkg/appbundle"
)

// projectHealth is the project checked as it stands, without moving the user:
// which steps have a problem, what Build would write, and which app it
// packages. It is recomputed only when the project changes.
type projectHealth struct {
	Issues  [tabCount]string // first problem per tab
	Outputs [tabCount]string // artifact path per tab, when known
	Count   int              // problems across all tabs
	App     appSummary
}

// appSummary describes the app bundle the project packages, read from its
// Info.plist.
type appSummary struct {
	Path, Name, Version, BundleID, Error string
}

// ready reports whether nothing stands in the way of a build.
func (h projectHealth) ready() bool { return h.Count == 0 }

// refreshHealth rechecks the project if it changed since the last check.
// Paths are checked on disk and the project is resolved, exactly as Validate
// does, but the result is only recorded, never shown on a field.
func (g *editor) refreshHealth() {
	if g.healthOf != nil && reflect.DeepEqual(g.healthOf, g.s.Project) {
		return
	}
	g.healthOf = g.s.Project.Clone()
	g.health = g.checkHealth()
}

// recheckHealth forces the next refresh, for when files on disk may have
// changed under an unchanged project, as after a build.
func (g *editor) recheckHealth() {
	g.healthOf = nil
	g.refreshHealth()
}

func (g *editor) checkHealth() projectHealth {
	var h projectHealth
	add := func(tab int, message string) {
		if h.Issues[tab] == "" {
			h.Issues[tab] = message
		}
		h.Count++
	}
	p := g.s.Project
	if p.Dep == nil && p.DMG == nil && p.PKG == nil {
		add(tabProject, "Enable DMG, PKG or Dependencies to build")
	}
	for _, problem := range g.pathProblems(false) {
		add(problem.tab, problem.label+": "+problem.err.Error())
	}
	plan, err := p.Resolve()
	if err != nil {
		if h.Count == 0 {
			add(g.issueTab(err), err.Error())
		}
	} else {
		if plan.DMG != nil {
			h.Outputs[tabDMG] = plan.DMG.FileName
		}
		if plan.PKG != nil {
			h.Outputs[tabPKG] = plan.PKG.Output
		}
	}
	h.App = g.summarizeApp(plan)
	return h
}

// summarizeApp reads the packaged app's Info.plist. A plan resolves variables
// in the path; without one the configured path is used as typed.
func (g *editor) summarizeApp(plan *zapp.Plan) appSummary {
	path := ""
	if plan != nil {
		path = plan.App
	} else if p := g.s.Project.App; p != "" && !strings.Contains(p, "${") {
		path = g.assetPath(p)
	}
	if path == "" {
		return appSummary{}
	}
	s := appSummary{Path: path, Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))}
	info, err := appbundle.Open(path)
	if err != nil {
		s.Error = "Info.plist could not be read"
		return s
	}
	if name, err := info.BundleName(); err == nil && name != "" {
		s.Name = name
	}
	s.Version, _ = info.Version()
	s.BundleID, _ = info.BundleID()
	return s
}

// projectIconKey is the asset key of the packaged app's icon on the Project tab.
const projectIconKey = "project:app"

// refreshProjectIcon loads the packaged app's icon for the Project tab. The DMG
// preview drops every non-cache key when it refreshes, so a missing icon is
// loaded again whenever the Project tab is rebuilt.
func (g *editor) refreshProjectIcon() {
	path := g.health.App.Path
	if path != g.projectIconPath {
		delete(g.assets, projectIconKey)
		g.projectIconPath = path
	}
	if g.tab != tabProject || path == "" || g.assets[projectIconKey] != nil {
		return
	}
	if g.appIconPaths == nil {
		g.appIconPaths = map[string]string{}
	}
	g.loadAppIcon(projectIconKey, path)
	g.assetsChanged()
}
