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
	if tab == tabDMG {
		g.dmgYAML = false
	}
	g.selected = ""
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

// pathFields returns the fields a section declares for validation. The DMG tab
// substitutes the whole form for a YAML editor field when dmgYAML is set, so
// validation always asks for the underlying form fields instead.
func (g *editor) pathFields(tab int, section section) []field {
	if tab == tabDMG {
		return g.dmgFormFields()
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
			if f.picker == "" || f.picker == pickSave || f.mayNotExist || f.Value == "" || strings.Contains(f.Value, "${") {
				continue
			}
			info, err := os.Stat(g.assetPath(f.Value))
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
	}
}
