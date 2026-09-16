package gui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// Validate checks build inputs, while Save continues to permit unfinished drafts.
func (g *editor) validatePaths() bool {
	if g.s.Project.DMG != nil {
		for path, item := range g.s.Project.DMG.Contents {
			if item.Icon == "" || strings.Contains(item.Icon, "${") {
				continue
			}
			if _, err := os.Stat(g.assetPath(item.Icon)); err != nil {
				g.tab = tabDMG
				g.selected = path
				g.rebuild()
				if len(g.inspector.Inputs) > itemIconFieldIndex {
					g.focus(g.inspectorStart + itemIconFieldIndex)
					g.fieldError(err)
				} else {
					g.report(err, "")
				}
				return false
			}
		}
	}
	for tab, section := range sections {
		if !section.Enabled(g.s.Project) {
			continue
		}
		fields := g.pathFields(tab, section)
		for _, f := range fields {
			if f.picker == "" || f.picker == pickSave || f.Value == "" || strings.Contains(f.Value, "${") {
				continue
			}
			info, err := os.Stat(g.assetPath(f.Value))
			if f.mayNotExist && os.IsNotExist(err) {
				continue
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
			if err != nil {
				if os.IsNotExist(err) {
					err = fmt.Errorf("Path does not exist: %s", f.Value)
				}
				g.showFieldError(tab, f.Label, err)
				return false
			}
		}
	}
	return true
}
func (g *editor) locateValidationError(err error) {
	var pathError *os.PathError
	if errors.As(err, &pathError) {
		for tab, section := range sections {
			if !section.Enabled(g.s.Project) {
				continue
			}
			fields := g.pathFields(tab, section)
			for _, f := range fields {
				if f.picker != "" && f.Value != "" && filepath.Clean(g.assetPath(f.Value)) == filepath.Clean(pathError.Path) {
					g.showFieldError(tab, f.Label, err)
					return
				}
			}
		}
	}
	if pathError != nil && g.s.Project.DMG != nil {
		for _, item := range g.s.layout().Items {
			if !item.Link && filepath.Clean(g.assetPath(item.Path)) == filepath.Clean(pathError.Path) {
				g.showFieldError(tabDMG, "Contents (JSON)", err)
				return
			}
		}
	}
	message := err.Error()
	if strings.HasPrefix(message, "app ") || strings.Contains(message, "requires app") || strings.Contains(message, "provide --app") {
		g.showFieldError(tabProject, "App bundle", err)
		return
	}
	switch {
	case strings.HasPrefix(message, "dep:") && g.s.Project.Dep != nil:
		g.showFieldError(tabDep, "", err)
	case (strings.HasPrefix(message, "pkg") || strings.Contains(message, "component") || strings.HasPrefix(message, "choice ")) && g.s.Project.PKG != nil:
		label := "Package type"
		if strings.Contains(message, "component id") {
			label = "Component ID"
			g.pkgRaw = false
			seen := map[string]bool{}
			for i, c := range g.s.Project.PKG.Components {
				if c.ID == "" || seen[c.ID] {
					g.componentIndex = i
					break
				}
				seen[c.ID] = true
			}
		}
		if strings.Contains(message, "full form requires components") {
			label = "Components"
		}
		if strings.Contains(message, "root must") || strings.Contains(message, "root is") {
			if !g.s.Project.PKG.HasFullForm() {
				g.showFieldError(tabProject, "App bundle", err)
				return
			}
			label = "Root directory"
			g.pkgRaw = false
			for i, c := range g.s.Project.PKG.Components {
				if c.Root == "" || strings.Contains(message, g.assetPath(c.Root)) {
					g.componentIndex = i
					break
				}
			}
		}
		if strings.Contains(message, "distribution") || strings.HasPrefix(message, "choice ") {
			label = "Distribution"
		}
		g.showFieldError(tabPKG, label, err)
	case strings.HasPrefix(message, "sign:") && g.s.Project.Sign != nil:
		g.showFieldError(tabSign, "", err)
	case strings.HasPrefix(message, "notarize:") && g.s.Project.Notarize != nil:
		g.showFieldError(tabNotarize, "", err)
	default:
		g.issue = &validationIssue{tab: g.tab, message: message}
	}
}
