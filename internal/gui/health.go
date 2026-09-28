package gui

import (
	"path/filepath"
	"reflect"
	"slices"
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

// refreshHealth rechecks the project if it changed since the last check.
// Paths are checked on disk and the project is resolved, exactly as Validate
// does, but the result is only recorded, never shown on a field.
func (g *editor) refreshHealth() {
	in := healthInputs(g.s.Project)
	if g.healthOf != nil && reflect.DeepEqual(g.healthOf, in) {
		return
	}
	g.healthOf = in
	g.health = g.checkHealth()
}

// healthInputs is the project as the health check reads it. Item positions
// only arrange the DMG window, so moving an icon does not recheck.
func healthInputs(p *zapp.Project) *zapp.Project {
	p = p.Clone()
	if p.DMG != nil {
		for path, item := range p.DMG.Contents {
			item.Pos = nil
			p.DMG.Contents[path] = item
		}
	}
	return p
}

// recheckHealth forces the next refresh, for when files on disk may have
// changed under an unchanged project, as after a build.
func (g *editor) recheckHealth() {
	g.healthOf, g.health.App, g.projectIconPath = nil, appSummary{}, ""
	g.refreshHealth()
	g.refreshProjectIcon()
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
	problems := g.pathProblems(false)
	for _, problem := range problems {
		add(problem.tab, problem.label+": "+problem.err.Error())
	}
	plan, err := p.Resolve()
	if err != nil {
		// A missing path usually fails Resolve too; count it once.
		loc, ok := g.locateIssue(err)
		if !ok {
			loc.tab = tabProject
		}
		if !slices.ContainsFunc(problems, func(p pathProblem) bool { return p.tab == loc.tab && (loc.label == "" || p.label == loc.label) }) {
			add(loc.tab, err.Error())
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

// summarizeApp reads the packaged app's Info.plist, once per app path until
// the next forced recheck. A plan resolves variables in the path; without one
// the configured path is used as typed.
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
	if path == g.health.App.Path {
		return g.health.App
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

// refreshProjectIcon loads the packaged app's icon once each time the app
// changes, and only for a bundle whose Info.plist could be read.
func (g *editor) refreshProjectIcon() {
	app := g.health.App
	if app.Path == g.projectIconPath {
		return
	}
	g.projectIconPath = app.Path
	delete(g.assets, projectIconKey)
	delete(g.appIconPaths, projectIconKey)
	if app.Path != "" && app.Error == "" {
		g.loadAppIcon(projectIconKey, app.Path)
	}
	g.pruneAssets()
	g.assetsChanged()
}
