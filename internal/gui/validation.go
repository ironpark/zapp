package gui

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// issueLocation is where a problem is fixed: a tab, the field on it when
// known, and the PKG component (or -1) and the DMG item (or "") the field
// belongs to.
type issueLocation struct {
	tab       int
	label     string
	component int
	item      string
}

// at is the field labelled label on tab, of no component or item.
func at(tab int, label string) issueLocation {
	return issueLocation{tab: tab, label: label, component: -1}
}

// describe names the field for a summary, with the component it is in.
func (l issueLocation) describe() string {
	if l.component >= 0 {
		return fmt.Sprintf("Component %d %s", l.component+1, strings.ToLower(l.label))
	}
	return l.label
}

// fieldLocation is where field i of the page is.
func (g *editor) fieldLocation(i int) issueLocation {
	loc := at(g.tab, g.fields[i].Label)
	if g.componentListVisible() {
		loc.component = g.componentIndex
	}
	if i >= g.inspectorStart {
		loc.item = g.selected
	}
	return loc
}

// fieldIndex is the index of the page's field labelled label, or -1.
func (g *editor) fieldIndex(label string) int {
	if label == "" {
		return -1
	}
	return slices.IndexFunc(g.fields, func(f field) bool { return f.Label == label })
}

func (g *editor) fieldError(err error) {
	if g.active >= 0 && g.active < len(g.fields) {
		g.issue = &validationIssue{g.fieldLocation(g.active), err.Error()}
		g.fields[g.active].Error = err.Error()
		g.revealField(g.active)
	}
	g.report(err, "")
}

func (g *editor) clearFieldError() {
	if g.active >= 0 && g.active < len(g.fields) {
		g.fields[g.active].Error = ""
	}
}

// showIssue moves the user to where err is fixed: its tab, opened so that
// the field shows, and the field, which shows err.
func (g *editor) showIssue(loc issueLocation, err error) {
	g.tab = loc.tab
	if loc.component >= 0 {
		g.componentIndex, g.pkgRaw = loc.component, false
	}
	switch loc.tab {
	case tabDMG:
		g.dmgYAML = false
	case tabPKG:
		g.pkgAdvanced = true
		if loc.label == labelComponents {
			g.pkgRaw = true
		}
	case tabDep:
		g.depRaw = false
	case tabSign:
		if method, ok := methodOwning(signMethodFields, loc.label); ok {
			g.signMode = method
		}
		g.signModeSet = true
	case tabNotarize:
		if method, ok := methodOwning(notaryMethodFields, loc.label); ok {
			g.notaryMode = method
		}
		g.notaryModeSet = true
	}
	g.selected = loc.item
	g.dmgAdvanced = true
	g.issue = &validationIssue{loc, err.Error()}
	g.rebuild()
	if i := g.fieldIndex(loc.label); i >= 0 {
		g.focus(i)
		g.fieldError(err)
		return
	}
	g.report(err, "")
}

// withFormView builds fields as if every collapsed or raw view were expanded, so
// validation sees the complete set no matter what the user is currently looking
// at, then restores the view the user had.
func (g *editor) withFormView(build func() []field) []field {
	dmgAdvanced, pkgAdvanced, depRaw, pkgRaw := g.dmgAdvanced, g.pkgAdvanced, g.depRaw, g.pkgRaw
	g.dmgAdvanced, g.pkgAdvanced, g.depRaw, g.pkgRaw = true, true, false, false
	defer func() {
		g.dmgAdvanced, g.pkgAdvanced, g.depRaw, g.pkgRaw = dmgAdvanced, pkgAdvanced, depRaw, pkgRaw
	}()
	return build()
}

// checkedField is a field validation checks, with where it is.
type checkedField struct {
	field
	issueLocation
}

// pathFields returns the fields a section declares for validation, as if
// every collapsed or raw view were expanded: the DMG tab's YAML editor
// stands for the form fields validation asks for instead.
func (g *editor) pathFields(tab int, section section) []checkedField {
	var fields []field
	switch tab {
	case tabDMG:
		fields = g.withFormView(g.dmgFormFields)
	case tabSign:
		fields = g.signAllFields()
	case tabNotarize:
		fields = g.notaryAllFields()
	case tabDep:
		fields = g.withFormView(g.depFields)
	case tabDistribution:
		fields = g.archiveFields()
	case tabPKG:
		c := g.s.Project.PKG
		if !c.HasFullForm() {
			fields = g.withFormView(g.pkgFields)
			break
		}
		var checked []checkedField
		for i := range c.Components {
			item := &c.Components[i]
			for _, f := range []field{pathField(labelRootDirectory, &item.Root, "", pickFolder), pathField(labelScriptsDirectory, &item.Scripts, "", pickFolder)} {
				loc := at(tab, f.Label)
				loc.component = i
				checked = append(checked, checkedField{f, loc})
			}
		}
		return checked
	default:
		fields = section.fields(g)
	}
	checked := make([]checkedField, len(fields))
	for i, f := range fields {
		checked[i] = checkedField{f, at(tab, f.Label)}
	}
	return checked
}

// pathProblem is a path setting that points at nothing usable.
type pathProblem struct {
	issueLocation
	err error
}

// pathProblems lists, without moving the user, every enabled path setting
// that points at nothing usable. It stops early when first is set.
func (g *editor) pathProblems(first bool) []pathProblem {
	var problems []pathProblem
	if g.s.Project.DMG != nil {
		for _, path := range slices.Sorted(maps.Keys(g.s.Project.DMG.Contents)) {
			item := g.s.Project.DMG.Contents[path]
			if item.Icon == "" || strings.Contains(item.Icon, "${") {
				continue
			}
			if _, err := os.Stat(g.assetPath(item.Icon)); err != nil {
				loc := at(tabDMG, labelItemIcon)
				loc.item = path
				problems = append(problems, pathProblem{loc, err})
				if first {
					return problems
				}
			}
		}
	}
	for tab, section := range sections {
		if !section.Enabled(g.s.Project) {
			continue
		}
		for _, f := range g.pathFields(tab, section) {
			if err := g.checkPath(f.field); err != nil {
				problems = append(problems, pathProblem{f.issueLocation, err})
				if first {
					return problems
				}
			}
		}
	}
	return problems
}

// checkPath reports why a path field's value is unusable, or nil.
func (g *editor) checkPath(f field) error {
	if f.picker == "" || f.picker == pickSave || f.Value == "" || strings.Contains(f.Value, "${") {
		return nil
	}
	info, err := os.Stat(g.assetPath(f.Value))
	if f.mayNotExist && os.IsNotExist(err) {
		return nil
	}
	if err == nil {
		switch f.picker {
		case pickFile, pickImage, pickIcon, pickItemIcon:
			if info.IsDir() {
				err = fmt.Errorf("Choose a file, not a directory")
			}
		case pickFolder:
			if !info.IsDir() {
				err = fmt.Errorf("Choose a directory")
			}
		case pickApp:
			if !info.IsDir() || !strings.HasSuffix(strings.ToLower(f.Value), ".app") {
				err = fmt.Errorf("Choose an .app bundle directory")
			}
		}
	}
	if os.IsNotExist(err) {
		err = fmt.Errorf("Path does not exist: %s", f.Value)
	}
	return err
}

// Validate checks build inputs, while Save continues to permit unfinished
// drafts. The first problem is shown on its field.
func (g *editor) validatePaths() bool {
	problems := g.pathProblems(true)
	if len(problems) == 0 {
		return true
	}
	g.showIssue(problems[0].issueLocation, problems[0].err)
	return false
}

// locateIssue finds, without moving the user, where err is fixed. It reports
// false for an error no tab owns.
func (g *editor) locateIssue(err error) (issueLocation, bool) {
	var pathError *os.PathError
	if errors.As(err, &pathError) {
		for tab, section := range sections {
			if !section.Enabled(g.s.Project) {
				continue
			}
			for _, f := range g.pathFields(tab, section) {
				if f.picker != "" && f.Value != "" && filepath.Clean(g.assetPath(f.Value)) == filepath.Clean(pathError.Path) {
					return f.issueLocation, true
				}
			}
		}
		if g.s.Project.DMG != nil {
			for _, item := range g.s.layout().Items {
				if !item.Link && filepath.Clean(g.assetPath(item.Path)) == filepath.Clean(pathError.Path) {
					// A missing source has no field: select its item.
					loc := at(tabDMG, "")
					loc.item = item.Path
					return loc, true
				}
			}
		}
	}
	message := err.Error()
	if strings.HasPrefix(message, "app ") || strings.Contains(message, "requires app") || strings.Contains(message, "provide --app") {
		return at(tabProject, labelAppBundle), true
	}
	switch {
	case strings.HasPrefix(message, "dep:") && g.s.Project.Dep != nil:
		return at(tabDep, ""), true
	case (strings.HasPrefix(message, "pkg") || strings.Contains(message, "component") || strings.HasPrefix(message, "choice ")) && g.s.Project.PKG != nil:
		loc := at(tabPKG, labelPackageType)
		if strings.Contains(message, "component id") {
			loc.label = labelComponentID
			seen := map[string]bool{}
			for i, c := range g.s.Project.PKG.Components {
				if c.ID == "" || seen[c.ID] {
					loc.component = i
					break
				}
				seen[c.ID] = true
			}
		}
		if strings.Contains(message, "full form requires components") {
			loc.label = labelComponents
		}
		if strings.Contains(message, "root must") || strings.Contains(message, "root is") {
			if !g.s.Project.PKG.HasFullForm() {
				return at(tabProject, labelAppBundle), true
			}
			loc.label = labelRootDirectory
			for i, c := range g.s.Project.PKG.Components {
				if c.Root == "" || strings.Contains(message, g.assetPath(c.Root)) {
					loc.component = i
					break
				}
			}
		}
		if strings.Contains(message, "distribution") || strings.HasPrefix(message, "choice ") {
			loc.label = labelDistribution
		}
		return loc, true
	case strings.HasPrefix(message, "sign:") && g.s.Project.Sign != nil:
		return at(tabSign, ""), true
	case strings.HasPrefix(message, "notarize timeout") && g.s.Project.Notarize != nil:
		return at(tabNotarize, labelNotaryTimeout), true
	case strings.HasPrefix(message, "notarize:") && g.s.Project.Notarize != nil:
		return at(tabNotarize, ""), true
	case strings.HasPrefix(message, "upload"):
		return at(tabDistribution, labelUploads), true
	case strings.HasPrefix(message, "appcast"):
		return at(tabDistribution, labelAppcast), true
	case strings.HasPrefix(message, "homebrew"):
		return at(tabDistribution, labelHomebrew), true
	case strings.HasPrefix(message, "zip"):
		return at(tabDistribution, labelZIPOutput), true
	case strings.HasPrefix(message, "checksums"):
		return at(tabDistribution, labelChecksumsOutput), true
	case strings.HasPrefix(message, "dmg") && g.s.Project.DMG != nil:
		return at(tabDMG, ""), true
	}
	return issueLocation{}, false
}

// locateValidationError moves the user to where err is fixed.
func (g *editor) locateValidationError(err error) {
	loc, ok := g.locateIssue(err)
	if !ok {
		g.issue = &validationIssue{at(g.tab, ""), err.Error()}
		return
	}
	g.showIssue(loc, err)
}
