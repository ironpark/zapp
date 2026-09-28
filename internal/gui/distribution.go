package gui

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/ironpark/zapp"
)

func (g *editor) distributionFields() []field {
	p, stash := g.s.Project, &g.disabled
	fields := []field{presenceField("Archive as ZIP", &p.Zip, &stash.Zip, "The notarized, stapled app, ready to download")}
	if p.Zip != nil {
		fields = append(fields, pathField("ZIP output", &p.Zip.Out, "Blank uses <output>/<app name>.zip", pickSave))
	}
	fields = append(fields, presenceField("Checksums", &p.Checksums, &stash.Checksums, "SHA256SUMS for the ZIP, DMG and PKG"))
	if p.Checksums != nil {
		fields = append(fields, pathField("Checksums output", &p.Checksums.Out, "Blank uses <output>/SHA256SUMS", pickSave))
	}
	return append(fields, g.uploadsField(), g.appcastField())
}

// presenceField switches an optional part on, with what it held when it was
// switched off, or off.
func presenceField[T any](label string, live, stash **T, hint string) field {
	f := field{Boolean: true, Label: label, Value: strconv.FormatBool(*live != nil), Hint: hint, Choices: []string{"false", "true"}}
	f.set = func(s string) error {
		on, err := strconv.ParseBool(strings.TrimSpace(s))
		if err != nil {
			return fmt.Errorf("%s must be true or false", label)
		}
		if on != (*live != nil) {
			toggleSection(live, stash)
		}
		return nil
	}
	return f
}

func (g *editor) uploadsField() field {
	value := ""
	if len(g.s.Project.Upload) > 0 {
		value = marshalYAML(g.s.Project.Upload)
	}
	return field{Label: "Uploads", Value: value, Multiline: true, Syntax: "yaml", Height: 170, Placeholder: "- url: https://example.com/${file.name}",
		Hint: "A list of endpoints, each a url or a github release; blank uploads nothing", set: func(value string) error {
			p, err := g.parseSection("upload", value)
			if err != nil {
				return err
			}
			g.s.Project.Upload = p.Upload
			return nil
		}}
}

func (g *editor) appcastField() field {
	value := ""
	if g.s.Project.Appcast != nil {
		value = marshalYAML(g.s.Project.Appcast)
	}
	return field{Label: "Sparkle appcast", Value: value, Multiline: true, Syntax: "yaml", Height: 130, Placeholder: "url: https://example.com/${file.name}",
		Hint: "Signed with ZAPP_SPARKLE_KEY from the environment; blank publishes none", set: func(value string) error {
			p, err := g.parseSection("appcast", value)
			if err != nil {
				return err
			}
			g.s.Project.Appcast = p.Appcast
			return nil
		}}
}

func marshalYAML(v any) string {
	data, _ := yaml.Marshal(v)
	return strings.TrimSuffix(string(data), "\n")
}

// parseSection reads one section of the project as YAML, checked exactly as
// a project file's would be. Blank means the section is absent.
func (g *editor) parseSection(key, value string) (*zapp.Project, error) {
	if strings.TrimSpace(value) == "" {
		return &zapp.Project{}, nil
	}
	var section any
	decoder := yaml.NewDecoder(strings.NewReader(value))
	if err := decoder.Decode(&section); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("enter exactly one YAML document")
	}
	doc, err := yaml.Marshal(map[string]any{"version": 1, key: section})
	if err != nil {
		return nil, err
	}
	return zapp.Parse(strings.NewReader(string(doc)), filepath.Dir(g.s.Path))
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
	if p.Appcast != nil {
		parts = append(parts, "appcast")
	}
	switch n := len(p.Upload); n {
	case 0:
	case 1:
		parts = append(parts, "1 upload")
	default:
		parts = append(parts, strconv.Itoa(n)+" uploads")
	}
	return strings.Join(parts, " · ")
}
