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

func (g *editor) fieldError(err error) {
	if g.active >= 0 && g.active < len(g.fields) {
		g.issue = &validationIssue{tab: g.tab, label: g.fields[g.active].Label, message: err.Error(), component: g.componentIndex, item: g.selected}
		g.fields[g.active].Error = err.Error()
		g.input.Spec.Error = err.Error()
		g.syncForm()
		g.revealField(g.active)
	}
	g.report(err, "")
}
func (g *editor) clearFieldError() {
	if g.active >= 0 && g.active < len(g.fields) {
		g.fields[g.active].Error = ""
		g.input.Spec.Error = ""
		g.syncForm()
	}
}
func (g *editor) showFieldError(tab int, label string, err error) {
	g.tab = tab
	if tab == tabDep {
		g.depRaw = false
	}
	if tab == tabPKG {
		g.pkgAdvanced = true
		var index int
		var kind string
		if _, e := fmt.Sscanf(label, "Component %d %s", &index, &kind); e == nil {
			g.componentIndex = index - 1
			g.pkgRaw = false
			if kind == "root" {
				label = "Root directory"
			} else if kind == "scripts" {
				label = "Scripts directory"
			}
		}
		if label == "Components" {
			g.pkgRaw = true
		}
	}
	if tab == tabSign {
		switch label {
		case "Signing identity":
			g.signMode = 0
		case "PKCS#12 certificate", "Password file":
			g.signMode = 1
		case "PEM certificate":
			g.signMode = 2
		}
		g.signModeSet = true
	}
	if tab == tabNotarize {
		switch label {
		case "Keychain profile":
			g.notaryMode = 0
		case "Apple ID", "Team ID", "App-specific password":
			g.notaryMode = 1
		case "API key file":
			g.notaryMode = 2
		}
		g.notaryModeSet = true
	}
	g.issue = &validationIssue{tab: tab, label: label, message: err.Error(), component: g.componentIndex, item: g.selected}
	if tab == tabDMG {
		g.dmgYAML = false
	}
	if tab != tabDMG || (label != "Name" && label != "X" && label != "Y" && label != "Item icon") {
		g.selected = ""
	}
	g.dmgAdvanced = true
	g.rebuild()
	for i, f := range g.fields {
		if f.Label == label {
			g.focus(i)
			g.fieldError(err)
			return
		}
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

// pathFields returns the fields a section declares for validation. The DMG tab
// substitutes the whole form for a YAML editor field when dmgYAML is set, so
// validation always asks for the underlying form fields instead.
func (g *editor) pathFields(tab int, section section) []field {
	switch tab {
	case tabDMG:
		return g.withFormView(g.dmgFormFields)
	case tabSign:
		return g.signAllFields()
	case tabNotarize:
		return g.notaryAllFields()
	case tabDep:
		return g.withFormView(g.depFields)
	case tabPKG:
		c := g.s.Project.PKG
		if !c.HasFullForm() {
			return g.withFormView(g.pkgFields)
		}
		var fields []field
		for i := range c.Components {
			item := &c.Components[i]
			fields = append(fields, pathField(fmt.Sprintf("Component %d root", i+1), &item.Root, "", pickFolder), pathField(fmt.Sprintf("Component %d scripts", i+1), &item.Scripts, "", pickFolder))
		}
		return fields
	}
	return section.fields(g)
}

// pathProblem is a path setting that points at nothing usable. item names the
// DMG content whose custom icon is missing; otherwise label names the field.
type pathProblem struct {
	tab         int
	label, item string
	err         error
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
				problems = append(problems, pathProblem{tab: tabDMG, label: "Item icon", item: path, err: err})
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
			if err := g.checkPath(f); err != nil {
				problems = append(problems, pathProblem{tab: tab, label: f.Label, err: err})
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
	p := problems[0]
	if p.item == "" {
		g.showFieldError(p.tab, p.label, p.err)
		return false
	}
	g.tab = tabDMG
	g.selected = p.item
	g.rebuild()
	if len(g.fields)-g.inspectorStart > itemIconFieldIndex {
		g.focus(g.inspectorStart + itemIconFieldIndex)
		g.fieldError(p.err)
	} else {
		g.report(p.err, "")
	}
	return false
}

// issueLocation is where a Resolve error is fixed: a tab, the field on it
// when known, and the PKG component to open, or -1.
type issueLocation struct {
	tab       int
	label     string
	component int
}

// locateIssue finds, without moving the user, where err is fixed. It reports
// false for an error no tab owns.
func (g *editor) locateIssue(err error) (issueLocation, bool) {
	at := func(tab int, label string) (issueLocation, bool) {
		return issueLocation{tab: tab, label: label, component: -1}, true
	}
	var pathError *os.PathError
	if errors.As(err, &pathError) {
		for tab, section := range sections {
			if !section.Enabled(g.s.Project) {
				continue
			}
			for _, f := range g.pathFields(tab, section) {
				if f.picker != "" && f.Value != "" && filepath.Clean(g.assetPath(f.Value)) == filepath.Clean(pathError.Path) {
					return at(tab, f.Label)
				}
			}
		}
		if g.s.Project.DMG != nil {
			for _, item := range g.s.layout().Items {
				if !item.Link && filepath.Clean(g.assetPath(item.Path)) == filepath.Clean(pathError.Path) {
					return at(tabDMG, "Contents (JSON)")
				}
			}
		}
	}
	message := err.Error()
	if strings.HasPrefix(message, "app ") || strings.Contains(message, "requires app") || strings.Contains(message, "provide --app") {
		return at(tabProject, "App bundle")
	}
	switch {
	case strings.HasPrefix(message, "dep:") && g.s.Project.Dep != nil:
		return at(tabDep, "")
	case (strings.HasPrefix(message, "pkg") || strings.Contains(message, "component") || strings.HasPrefix(message, "choice ")) && g.s.Project.PKG != nil:
		loc, _ := at(tabPKG, "Package type")
		if strings.Contains(message, "component id") {
			loc.label = "Component ID"
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
			loc.label = "Components"
		}
		if strings.Contains(message, "root must") || strings.Contains(message, "root is") {
			if !g.s.Project.PKG.HasFullForm() {
				return at(tabProject, "App bundle")
			}
			loc.label = "Root directory"
			for i, c := range g.s.Project.PKG.Components {
				if c.Root == "" || strings.Contains(message, g.assetPath(c.Root)) {
					loc.component = i
					break
				}
			}
		}
		if strings.Contains(message, "distribution") || strings.HasPrefix(message, "choice ") {
			loc.label = "Distribution"
		}
		return loc, true
	case strings.HasPrefix(message, "sign:") && g.s.Project.Sign != nil:
		return at(tabSign, "")
	case strings.HasPrefix(message, "notarize timeout") && g.s.Project.Notarize != nil:
		return at(tabNotarize, "Timeout")
	case strings.HasPrefix(message, "notarize:") && g.s.Project.Notarize != nil:
		return at(tabNotarize, "")
	case strings.HasPrefix(message, "upload"):
		return at(tabDistribution, "Uploads")
	case strings.HasPrefix(message, "appcast"):
		return at(tabDistribution, "Sparkle appcast")
	case strings.HasPrefix(message, "zip"):
		return at(tabDistribution, "ZIP output")
	case strings.HasPrefix(message, "checksums"):
		return at(tabDistribution, "Checksums output")
	case strings.HasPrefix(message, "dmg") && g.s.Project.DMG != nil:
		return at(tabDMG, "")
	}
	return issueLocation{}, false
}

// locateValidationError moves the user to where err is fixed.
func (g *editor) locateValidationError(err error) {
	loc, ok := g.locateIssue(err)
	if !ok {
		g.issue = &validationIssue{tab: g.tab, message: err.Error()}
		return
	}
	if loc.component >= 0 {
		g.componentIndex, g.pkgRaw = loc.component, false
	}
	g.showFieldError(loc.tab, loc.label, err)
}
