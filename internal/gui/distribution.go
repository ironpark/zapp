package gui

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/ironpark/zapp"
)

func (g *editor) distributionFields() []field {
	return append(g.archiveFields(), g.uploadsField(), g.appcastField(), g.homebrewField())
}

// archiveFields are the ZIP, checksums and verify settings, which hold the
// tab's only paths.
func (g *editor) archiveFields() []field {
	p, stash := g.s.Project, &g.disabled
	fields := []field{presenceField("Archive as ZIP", &p.Zip, &stash.Zip, "The notarized, stapled app, ready to download")}
	if p.Zip != nil {
		fields = append(fields, pathField("ZIP output", &p.Zip.Out, "Blank uses <output>/<app name>.zip", pickSave))
	}
	fields = append(fields, presenceField("Checksums", &p.Checksums, &stash.Checksums, "SHA256SUMS for the ZIP, DMG and PKG"))
	if p.Checksums != nil {
		fields = append(fields, pathField("Checksums output", &p.Checksums.Out, "Blank uses <output>/SHA256SUMS", pickSave))
	}
	return append(fields, boolField("Verify before publishing", &p.Verify, "Stop unless the app and every artifact are signed, stapled and accepted"))
}

// presenceField switches an optional part on, with what it held when it was
// switched off, or off.
func presenceField[T any](label string, live, stash **T, hint string) field {
	on := *live != nil
	f := boolField(label, &on, hint)
	set := f.set
	f.set = func(s string) error {
		if err := set(s); err != nil {
			return err
		}
		if on != (*live != nil) {
			toggleSection(live, stash)
		}
		return nil
	}
	return f
}

func (g *editor) uploadsField() field {
	p := g.s.Project
	return g.sectionField("Uploads", "upload", len(p.Upload) > 0, p.Upload, "- url: https://example.com/${file.name}",
		"A list of endpoints, each a url or a github release; blank uploads nothing",
		func(parsed *zapp.Project) { p.Upload = parsed.Upload })
}

func (g *editor) appcastField() field {
	p := g.s.Project
	return g.sectionField("Sparkle appcast", "appcast", p.Appcast != nil, p.Appcast, "url: https://example.com/${file.name}",
		"Signed with ZAPP_SPARKLE_KEY from the environment; blank publishes none",
		func(parsed *zapp.Project) { p.Appcast = parsed.Appcast })
}

func (g *editor) homebrewField() field {
	p := g.s.Project
	return g.sectionField("Homebrew cask", "homebrew", p.Homebrew != nil, p.Homebrew, "homepage: https://example.com\ntap: owner/homebrew-tap",
		"Committed to the tap with ZAPP_HOMEBREW_TOKEN or GITHUB_TOKEN; blank writes none",
		func(parsed *zapp.Project) { p.Homebrew = parsed.Homebrew })
}

// sectionField edits the project's key section as YAML, checked exactly as
// a project file's would be; use takes the project parsed from it. Blank
// means the section is absent.
func (g *editor) sectionField(label, key string, present bool, current any, placeholder, hint string, use func(*zapp.Project)) field {
	value := ""
	if present {
		value = marshalYAML(current)
	}
	return field{Label: label, Value: value, Multiline: true, Syntax: "yaml", Placeholder: placeholder, Hint: hint, set: func(value string) error {
		parsed := &zapp.Project{}
		if strings.TrimSpace(value) != "" {
			var section any
			if err := decodeOneYAML(value, &section); err != nil {
				return err
			}
			doc, err := yaml.Marshal(map[string]any{"version": 1, key: section})
			if err != nil {
				return err
			}
			if parsed, err = zapp.Parse(bytes.NewReader(doc), filepath.Dir(g.s.Path)); err != nil {
				return err
			}
		}
		use(parsed)
		return nil
	}}
}

// distributionSummary lists the distribution parts switched on, or nothing.
func distributionSummary(p *zapp.Project) string {
	var parts []string
	if p.Zip != nil {
		parts = append(parts, "ZIP")
	}
	if p.Checksums != nil {
		parts = append(parts, "checksums")
	}
	if p.Verify {
		parts = append(parts, "verify")
	}
	if p.Appcast != nil {
		parts = append(parts, "appcast")
	}
	if p.Homebrew != nil {
		parts = append(parts, "Homebrew")
	}
	if n := len(p.Upload); n > 0 {
		parts = append(parts, fmt.Sprintf("%d upload%s", n, plural(n)))
	}
	return strings.Join(parts, " · ")
}
